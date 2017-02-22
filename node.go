// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// node implements ipfs network transport for communicating between holochain nodes

package holochain

import (
	"context"
	_ "errors"
	//	host "github.com/libp2p/go-libp2p-host"
	"errors"
	ic "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
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

const (
	DHTProtocol    = protocol.ID("/holochain-dht/0.0.0")
	SourceProtocol = protocol.ID("/holochain-src/0.0.0")
)

// NewNode creates a new ipfs basichost node with given identity
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

// StartDHT initiates listening for DHT protocol messages on the node
func (node *Node) StartDHT() (err error) {
	node.Host.SetStreamHandler(DHTProtocol, func(s net.Stream) {
		//	defer s.Close()
	})
	return
}

// StartSrc initiates listening for Source protocol messages on the node
func (node *Node) StartSrc() (err error) {
	node.Host.SetStreamHandler(SourceProtocol, func(s net.Stream) {
		//	defer s.Close()
	})
	return
}

// Close shuts down the node
func (node *Node) Close() error {
	return node.Host.Close()
}

// Send delivers a message to a node via the given protocol
func (node *Node) Send(proto protocol.ID, addr peer.ID, data []byte) (err error) {
	s, err := node.Host.NewStream(context.Background(), addr, proto)
	if err != nil {
		return
	}
	defer s.Close()
	n, err := s.Write(data)
	if err != nil {
		return
	}
	if n != len(data) {
		err = errors.New("unable to send all data")
	}
	return
}
