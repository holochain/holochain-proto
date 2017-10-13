// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
// This code is adapted from the libp2p project, specifically:
// https://github.com/libp2p/go-libp2p-kad-dht/notif.go
//
// The ipfs use of kademlia is substantially different than that needed by holochain so we remove
// parts we don't need and add others.
//
package holochain

import (
	"context"
	inet "github.com/libp2p/go-libp2p-net"
	ma "github.com/multiformats/go-multiaddr"
)

// netNotifiee defines methods to be used with the Holochain Node
type netNotifiee Node

func (nn *netNotifiee) Node() *Node {
	return (*Node)(nn)
}

type peerTracker struct {
	refcount int
	cancel   func()
}

func (nn *netNotifiee) Connected(n inet.Network, v inet.Conn) {
	node := nn.Node()
	select {
	case <-node.Process().Closing():
		return
	default:
	}

	node.plk.Lock()
	defer node.plk.Unlock()

	conn, ok := nn.peers[v.RemotePeer()]
	if ok {
		conn.refcount++
		return
	}

	ctx, cancel := context.WithCancel(node.Context())

	nn.peers[v.RemotePeer()] = &peerTracker{
		refcount: 1,
		cancel:   cancel,
	}

	// Check if canceled under the lock.
	if ctx.Err() == nil {
		node.routingTable.Update(v.RemotePeer())
	}
}

func (nn *netNotifiee) Disconnected(n inet.Network, v inet.Conn) {
	node := nn.Node()
	select {
	case <-node.Process().Closing():
		return
	default:
	}

	node.plk.Lock()
	defer node.plk.Unlock()

	conn, ok := nn.peers[v.RemotePeer()]
	if !ok {
		// Unmatched disconnects are fine. It just means that we were
		// already connected when we registered the listener.
		return
	}
	conn.refcount -= 1
	if conn.refcount == 0 {
		delete(nn.peers, v.RemotePeer())
		conn.cancel()
		node.routingTable.Remove(v.RemotePeer())
	}
}

func (nn *netNotifiee) OpenedStream(n inet.Network, v inet.Stream) {}
func (nn *netNotifiee) ClosedStream(n inet.Network, v inet.Stream) {}
func (nn *netNotifiee) Listen(n inet.Network, a ma.Multiaddr)      {}
func (nn *netNotifiee) ListenClose(n inet.Network, a ma.Multiaddr) {}
