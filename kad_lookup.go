// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//
// This code is adapted from the libp2p project, specifically:
// https://github.com/libp2p/go-libp2p-kad-dht/lookup.go
// The ipfs use of kademlia is substantially different than that needed by holochain so we remove
// parts we don't need and add others.

package holochain

import (
	"context"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	. "github.com/metacurrency/holochain/hash"
	//	notif "github.com/libp2p/go-libp2p-routing/notifications"
	"errors"
)

var ErrLookupFailure = errors.New("node lookup failure")

func toPeerInfos(ps []peer.ID) []*pstore.PeerInfo {
	out := make([]*pstore.PeerInfo, len(ps))
	for i, p := range ps {
		out[i] = &pstore.PeerInfo{ID: p}
	}
	return out
}

// Kademlia 'node lookup' operation. Returns a channel of the K closest peers
// to the given key
func (node *Node) GetClosestPeers(ctx context.Context, key peer.ID) (<-chan peer.ID, error) {
	Debugf("Finding peers close to %v", key)
	tablepeers := node.routingTable.NearestPeers(key, AlphaValue)
	if len(tablepeers) == 0 {
		return nil, ErrLookupFailure
	}

	out := make(chan peer.ID, KValue)

	// since the query doesn't actually pass our context down
	// we have to hack this here. whyrusleeping isnt a huge fan of goprocess
	//parent := ctx
	query := node.newQuery(HashFromPeerID(key), func(ctx context.Context, p peer.ID) (*dhtQueryResult, error) {
		// For DHT query command
		/*notif.PublishQueryEvent(parent, &notif.QueryEvent{
			Type: notif.SendingQuery,
			ID:   p,
		})*/

		closer, err := node.closerPeersSingle(ctx, key, p)
		if err != nil {
			Debugf("error getting closer peers: %s", err)
			return nil, err
		}

		peerinfos := toPeerInfos(closer)

		// For DHT query command
		/*notif.PublishQueryEvent(parent, &notif.QueryEvent{
			Type:      notif.PeerResponse,
			ID:        p,
			Responses: peerinfos, // todo: remove need for this pointerize thing
		})*/

		return &dhtQueryResult{closerPeers: peerinfos}, nil
	})

	go func() {
		defer close(out)
		// run it!
		res, err := query.Run(ctx, tablepeers)
		if err != nil {
			Debugf("closestPeers query run error: %s", err)
		}

		if res != nil && res.finalSet != nil {
			sorted := SortClosestPeers(res.finalSet.Peers(), key)
			if len(sorted) > KValue {
				sorted = sorted[:KValue]
			}

			for _, p := range sorted {
				out <- p
			}
		}
	}()

	return out, nil
}

func (node *Node) closerPeersSingle(ctx context.Context, key peer.ID, p peer.ID) ([]peer.ID, error) {
	response, err := node.findPeerSingle(ctx, p, key)
	if err != nil {
		return nil, err
	}

	resp := response.Body.(FindNodeResp)
	var out []peer.ID
	for _, pinfo := range resp.CloserPeers {
		if pinfo.ID != node.HashAddr { // dont add self
			node.peerstore.AddAddrs(pinfo.ID, pinfo.Addrs, pstore.TempAddrTTL)
			out = append(out, pinfo.ID)
		}
	}
	return out, nil
}
