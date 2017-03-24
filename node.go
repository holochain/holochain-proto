// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// node implements ipfs network transport for communicating between holochain nodes

package holochain

import (
	"context"
	//	host "github.com/libp2p/go-libp2p-host"
	"encoding/gob"
	"errors"
	ic "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
	"io"
	"time"
)

type ReceiverFn func(h *Holochain, m *Message) (response interface{}, err error)

type MsgType int8

const (
	// common messages

	ERROR_RESPONSE MsgType = iota
	OK_RESPONSE

	// DHT messages

	PUT_REQUEST
	GET_REQUEST
	LINK_REQUEST
	GETLINK_REQUEST
	GOSSIP_REQUEST
	GOSSIP

	// Validate Messages

	VALIDATE_REQUEST
	VALIDATELINK_REQUEST
)

// Message represents data that can be sent to node in the network
type Message struct {
	Type MsgType
	Time time.Time
	From peer.ID
	Body interface{}
}

// Node represents a node in the network
type Node struct {
	HashAddr peer.ID
	NetAddr  ma.Multiaddr
	Host     *rhost.RoutedHost
}

// Protocol encapsulates data for our different protocols
type Protocol struct {
	ID       protocol.ID
	Receiver ReceiverFn
}

var DHTProtocol, ValidateProtocol, GossipProtocol Protocol

type Router struct {
	dummy int
}

func (r *Router) FindPeer(context.Context, peer.ID) (peer pstore.PeerInfo, err error) {
	err = errors.New("routing not implemented")
	return
}

// NewNode creates a new ipfs basichost node with given identity
func NewNode(listenAddr string, id peer.ID, priv ic.PrivKey) (node *Node, err error) {
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

	if pid.String() != id.String() {
		err = errors.New("NewNode: Id doesn't match key")
		return
	}

	n.HashAddr = pid
	ps.AddPrivKey(pid, priv)
	ps.AddPubKey(pid, priv.GetPublic())

	ctx := context.Background()

	// create a new swarm to be used by the service host
	netw, err := swarm.NewNetwork(ctx, []ma.Multiaddr{n.NetAddr}, pid, ps, nil)
	if err != nil {
		return nil, err
	}

	var bh *bhost.BasicHost
	bh, err = bhost.New(netw), nil
	if err != nil {
		return
	}
	hr := Router{}
	n.Host = rhost.Wrap(bh, &hr)

	node = &n
	return
}

// Encode codes a message to gob format
// @TODO generalize for other message encoding formats
func (m *Message) Encode() (data []byte, err error) {
	data, err = ByteEncoder(m)
	if err != nil {
		return
	}
	return
}

// Decode converts a message from gob format
// @TODO generalize for other message encoding formats
func (m *Message) Decode(r io.Reader) (err error) {
	dec := gob.NewDecoder(r)
	err = dec.Decode(m)
	return
}

// respondWith writes a message either error or otherwise, to the stream
func (node *Node) respondWith(s net.Stream, err error, body interface{}) {
	var m *Message
	if err != nil {
		m = node.NewMessage(ERROR_RESPONSE, err.Error())
	} else {
		m = node.NewMessage(OK_RESPONSE, body)
	}

	data, err := m.Encode()
	if err != nil {
		panic(err) //TODO can't panic, gotta do something else!
	}
	_, err = s.Write(data)
	if err != nil {
		panic(err) //TODO can't panic, gotta do something else!
	}
}

// StartProtocol initiates listening for a protocol on the node
func (node *Node) StartProtocol(h *Holochain, proto Protocol) (err error) {
	node.Host.SetStreamHandler(proto.ID, func(s net.Stream) {
		var m Message
		err := m.Decode(s)
		var response interface{}
		if m.From == "" {
			// @todo other sanity checks on From?
			err = errors.New("message must have a source")
		} else {
			if err == nil {
				response, err = proto.Receiver(h, &m)
			}
		}
		node.respondWith(s, err, response)
	})
	return
}

// Close shuts down the node
func (node *Node) Close() error {
	return node.Host.Close()
}

// Send delivers a message to a node via the given protocol
func (node *Node) Send(proto Protocol, addr peer.ID, m *Message) (response Message, err error) {
	s, err := node.Host.NewStream(context.Background(), addr, proto.ID)
	if err != nil {
		return
	}
	defer s.Close()

	// encode the message and send it
	data, err := m.Encode()
	if err != nil {
		return
	}

	n, err := s.Write(data)
	if err != nil {
		return
	}
	if n != len(data) {
		err = errors.New("unable to send all data")
	}

	// decode the response
	err = response.Decode(s)
	if err != nil {
		return
	}
	return
}

// NewMessage creates a message from the node with a new current timestamp
func (node *Node) NewMessage(t MsgType, body interface{}) (msg *Message) {
	m := Message{Type: t, Time: time.Now(), Body: body, From: node.HashAddr}
	msg = &m
	return
}
