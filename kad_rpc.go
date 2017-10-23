// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//
// This code is adapted from the libp2p project, specifically:
// https://github.com/libp2p/go-libp2p-kad-dht/routing.go
// The ipfs use of kademlia is substantially different than that needed by holochain so we remove
// parts we don't need and add others, also we have do our message wire-formats and encoding
// differently, so our RPC handlers are need to be different.

package holochain

import (
	"context"
	"errors"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	routing "github.com/libp2p/go-libp2p-routing"
	. "github.com/metacurrency/holochain/hash"
	ma "github.com/multiformats/go-multiaddr"
	_ "sync"
	_ "time"
)

var ErrDHTUnexpectedTypeInBody error = errors.New("unexpected type in message body")

type FindNodeReq struct {
	H Hash
}

// an encodable version of pstore.PeerInfo which gob doesn't like
// also libp2p encodes other stuff like connection type into this
// which we may have to do too.
type PeerInfo struct {
	ID    []byte   // byte version peer.ID
	Addrs [][]byte // byte version of multiaddrs
}

type CloserPeersResp struct {
	CloserPeers []PeerInfo // note this is not a pstore.PeerInfo which can't be serialized by gob.
}

// The number of closer peers to send on requests.
var CloserPeerCount = KValue

// FindLocal looks for a peer with a given ID connected to this node and returns its peer info
func (node *Node) FindLocal(id peer.ID) pstore.PeerInfo {
	p := node.routingTable.Find(id)
	if p != "" {
		return node.peerstore.PeerInfo(p)
	}
	return pstore.PeerInfo{}
}

// findPeerSingle asks peer 'p' if they know where the peer with id 'id' is and respond with
// any closer peers if not.
func (node *Node) findPeerSingle(ctx context.Context, p peer.ID, hash Hash) (closerPeers []*pstore.PeerInfo, err error) {
	node.log.Logf("Sending FIND_NODE_REQUEST to %v for hash: %v\n", p, hash)
	pmes := node.NewMessage(FIND_NODE_REQUEST, FindNodeReq{H: hash})
	var resp Message
	resp, err = node.Send(ctx, KademliaProtocol, p, pmes)
	if err != nil {
		return
	}

	response, ok := resp.Body.(CloserPeersResp)
	if !ok {
		err = ErrDHTUnexpectedTypeInBody
		return
	}

	closerPeers = peerInfos2Pis(response.CloserPeers)

	return
}

// nearestPeersToHash returns the routing tables closest peers to a given hash
func (node *Node) nearestPeersToHash(hash *Hash, count int) []peer.ID {
	closer := node.routingTable.NearestPeers(*hash, count)
	return closer
}

// betterPeersForHash returns nearestPeersToHash, but iff closer than self.
func (node *Node) betterPeersForHash(hash *Hash, p peer.ID, count int) []peer.ID {
	closer := node.nearestPeersToHash(hash, count)

	// no node? nil
	if closer == nil {
		node.log.Logf("no closer peers to send to %v", p)
		return nil
	}

	var filtered []peer.ID
	for _, clp := range closer {

		// == to self? thats bad
		if clp == node.HashAddr {
			Debug("attempted to return self! this shouldn't happen...")
			return nil
		}
		// Dont send a peer back themselves
		if clp == p {
			continue
		}

		filtered = append(filtered, clp)
	}

	// ok seems like closer nodes
	return filtered
}

// FindPeer searches for a peer with given ID.
// it is also an implementation the FindPeer() method of the RoutedHost interface in go-libp2p/p2p/host/routed
// and makes the Node object the "Router"
func (node *Node) FindPeer(ctx context.Context, id peer.ID) (pstore.PeerInfo, error) {
	// Check if were already connected to them
	if pi := node.FindLocal(id); pi.ID != "" {
		return pi, nil
	}

	hashID := HashFromPeerID(id)
	peers := node.routingTable.NearestPeers(hashID, AlphaValue)
	if len(peers) == 0 {
		return pstore.PeerInfo{}, ErrEmptyRoutingTable
	}

	// Sanity...
	for _, p := range peers {
		if p == id {
			node.log.Logf("found target peer in list of closest peers...")
			return node.peerstore.PeerInfo(p), nil
		}
	}

	// setup the Query
	query := node.newQuery(hashID, func(ctx context.Context, p peer.ID) (*dhtQueryResult, error) {
		/*	notif.PublishQueryEvent(parent, &notif.QueryEvent{
			Type: notif.SendingQuery,
			ID:   p,
		})*/

		closerPeers, err := node.findPeerSingle(ctx, p, hashID)
		if err != nil {
			return nil, err
		}

		// see if we got the peer here
		for _, npi := range closerPeers {
			if npi.ID == id {
				return &dhtQueryResult{
					peer:    npi,
					success: true,
				}, nil
			}
		}

		/*		notif.PublishQueryEvent(parent, &notif.QueryEvent{
				Type:      notif.PeerResponse,
				ID:        p,
				Responses: clpeerInfos,
			})*/

		return &dhtQueryResult{closerPeers: closerPeers}, nil
	})

	// run it!
	result, err := query.Run(ctx, peers)
	if err != nil {
		return pstore.PeerInfo{}, err
	}

	node.log.Logf("FindPeer %v %v", id, result.success)
	if result.peer.ID == "" {
		return pstore.PeerInfo{}, routing.ErrNotFound
	}

	return *result.peer, nil
}

// KademliaReceiver implements the handler for the kademlia RPC protocol messages
func KademliaReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	dht := h.dht
	node := h.node
	switch m.Type {
	case FIND_NODE_REQUEST:
		dht.dlog.Logf("KademliaReceiver got: %v", m)
		switch t := m.Body.(type) {
		case FindNodeReq:

			p := m.From
			var closest []peer.ID
			resp := CloserPeersResp{}
			// if looking for self... special case where we send it on CloserPeers.
			x := HashFromPeerID(node.HashAddr)
			if x.Equal(&t.H) {
				closest = []peer.ID{node.HashAddr}
			} else {
				closest = node.betterPeersForHash(&t.H, p, CloserPeerCount)
			}
			if closest == nil {
				dht.dlog.Logf("could not find any peers")
				return &resp, nil
			}

			resp.CloserPeers = node.peers2PeerInfos(closest)
			response = &resp
		default:
			err = ErrDHTUnexpectedTypeInBody
		}
	default:
		err = fmt.Errorf("message type %d not in holochain-kademlia protocol", int(m.Type))
	}
	return
}

// PI2PeerInfos convert the closest PeerInfos to a serializable type and gets their addrs from the peerstore
func (node *Node) peers2PeerInfos(peers []peer.ID) []PeerInfo {
	var pis []PeerInfo
	infos := pstore.PeerInfos(node.peerstore, peers)
	for _, pi := range infos {
		if len(pi.Addrs) > 0 {
			addrs := make([][]byte, len(pi.Addrs))
			for i, a := range pi.Addrs {
				addrs[i] = a.Bytes()
			}
			pis = append(pis, PeerInfo{ID: []byte(pi.ID), Addrs: addrs})
		}
	}
	return pis
}

// PeerInfos2Pis convert a list of PeerInfo structs to a list of pstore.PeerInfo
func peerInfos2Pis(peerInfos []PeerInfo) []*pstore.PeerInfo {
	pis := make([]*pstore.PeerInfo, 0, len(peerInfos))
	for _, pi := range peerInfos {
		peerInfo := pstore.PeerInfo{ID: peer.ID(pi.ID)}
		if len(pi.Addrs) > 0 {
			maddrs := make([]ma.Multiaddr, 0, len(pi.Addrs))
			for _, addr := range pi.Addrs {
				maddr, err := ma.NewMultiaddrBytes(addr)
				if err != nil {
					Infof("error decoding Multiaddr for peer: %s", peerInfo.ID)
					continue
				}
				maddrs = append(maddrs, maddr)
			}
			peerInfo.Addrs = maddrs
		}
		pis = append(pis, &peerInfo)
	}
	return pis
}
