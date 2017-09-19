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
	. "github.com/metacurrency/holochain/hash"
	_ "sync"
	_ "time"
)

type FindNodeReq struct {
	hash Hash
}

type FindNodeResp struct {
	CloserPeers []pstore.PeerInfo
}

// FindLocal looks for a peer with a given ID connected to this node and returns its peer info
func (node *Node) FindLocal(id peer.ID) pstore.PeerInfo {
	p := node.routingTable.Find(id)
	if p != "" {
		return node.peerstore.PeerInfo(p)
	}
	return pstore.PeerInfo{}
}

// findPeerSingle asks peer 'p' if they know where the peer with id 'id' is
func (node *Node) findPeerSingle(ctx context.Context, p peer.ID, hash Hash) (response *Message, err error) {

	pmes := node.NewMessage(FIND_NODE_REQUEST, FindNodeReq{hash: hash})
	var resp Message
	resp, err = node.Send(ctx, KademliaProtocol, p, pmes)
	if err != nil {
		return
	}
	response = &resp
	return
}

/*
// FindPeer searches for a peer with given ID.
func (node *Node) FindPeer(ctx context.Context, id peer.ID) (pstore.PeerInfo, error) {

	// Check if were already connected to them
	if pi := node.FindLocal(id); pi.ID != "" {
		return pi, nil
	}

	peers := node.routingTable.NearestPeers(id, AlphaValue)
	if len(peers) == 0 {
		return pstore.PeerInfo{}, ErrLookupFailure
	}

	// Sanity...
	for _, p := range peers {
		if p == id {
			Debug("found target peer in list of closest peers...")
			return node.peerstore.PeerInfo(p), nil
		}
	}

	// setup the Query
	parent := ctx
	query := dht.newQuery(string(id), func(ctx context.Context, p peer.ID) (*dhtQueryResult, error) {
		/*	notif.PublishQueryEvent(parent, &notif.QueryEvent{
			Type: notif.SendingQuery,
			ID:   p,
		})*/
/*
	pmes, err := node.findPeerSingle(ctx, p, id)
	if err != nil {
		return nil, err
	}

	closer := pmes.GetCloserPeers()
	clpeerInfos := pb.PBPeersToPeerInfos(closer)

	// see if we got the peer here
	for _, npi := range clpeerInfos {
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
/*
		return &dhtQueryResult{closerPeers: clpeerInfos}, nil
	})

	// run it!
	result, err := query.Run(ctx, peers)
	if err != nil {
		return pstore.PeerInfo{}, err
	}

	Debugf("FindPeer %v %v", id, result.success)
	if result.peer.ID == "" {
		return pstore.PeerInfo{}, routing.ErrNotFound
	}

	return *result.peer, nil
}
*/
// KademliaReceiver implements the handler for the kademlia RPC protocol messages
func KademliaReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	dht := h.dht
	switch m.Type {
	case FIND_NODE_REQUEST:
		dht.dlog.Logf("KademliaReceiver got FIND_NODE_REQUEST: %v", m)
		return nil, errors.New("notImplemented")
	default:
		err = fmt.Errorf("message type %d not in holochain-kademlia protocol", int(m.Type))
	}
	return
}
