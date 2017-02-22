// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// node implements ipfs network transport for communicating between holochain nodes

package holochain

import (
	"context"
	_ "errors"
	//	host "github.com/libp2p/go-libp2p-host"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
)

// Node represents a node in the network
type Node struct {
	HashAddr peer.ID
	NetAddr  ma.Multiaddr
	Host     *bhost.BasicHost
}

func NewNode(listenAddr string, priv ic.PrivKey) (node *Node, err error) {
	var n Node
	n.NetAddr, err = ma.NewMultiaddr(listenAddr)
	if err != nil {
		return
	}

	ps := pstore.NewPeerstore()
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return
	}
	n.HashAddr = pid
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, priv.GetPublic())

	ctx := context.Background()

	// create a new swar m to be used by the service host
	netw, err := swarm.NewNetwork(ctx, []ma.Multiaddr{n.NetAddr}, pid, ps, nil)
	if err != nil {
		return nil, err
	}

	n.Host, err = bhost.New(netw), nil
	if err != nil {
		return
	}
	node = &n
	return
}
