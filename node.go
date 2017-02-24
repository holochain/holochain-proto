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
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
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

	// Source Messages

	SRC_VALIDATE
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

// Encode codes a message to gob format @TODO generalize
func (m *Message) Encode() (data []byte, err error) {
	data, err = ByteEncoder(m)
	if err != nil {
		return
	}
	return
}

// Decode converts a message from gob format @TODO generalize
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
func (node *Node) StartProtocol(h *Holochain, proto protocol.ID, receiver ReceiverFn) (err error) {
	node.Host.SetStreamHandler(proto, func(s net.Stream) {
		var m Message
		err := m.Decode(s)
		var response interface{}
		if m.From == "" {
			// @todo other sanity checks on From?
			err = errors.New("message must have a source")
		} else {
			if err == nil {
				response, err = receiver(h, &m)
			}
		}
		node.respondWith(s, err, response)
	})
	return
}

// SrcReceiver handles messages on the Source protocol
func SrcReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	switch m.Type {
	case SRC_VALIDATE:
		switch t := m.Body.(type) {
		case Hash:
			response, err = h.store.GetEntry(t)
		default:
			err = errors.New("expected hash")
		}
	default:
		err = fmt.Errorf("message type %d not in holochain-src protocol", int(m.Type))
	}
	return
}

// StartSrc initiates listening for Source protocol messages on the node
func (node *Node) StartSrc(h *Holochain) (err error) {
	return node.StartProtocol(h, SourceProtocol, SrcReceiver)
}

// Close shuts down the node
func (node *Node) Close() error {
	return node.Host.Close()
}

// Send builds a message and either delivers it locally or via node.Send
func (h *Holochain) Send(proto protocol.ID, to peer.ID, t MsgType, body interface{}, receiver ReceiverFn) (response interface{}, err error) {
	message := h.node.NewMessage(t, body)
	if err != nil {
		return
	}
	// if we are sending to ourselves we should bypass the network mechanics and call
	// the receiver directly
	if to == h.node.HashAddr {
		response, err = receiver(h, message)
	} else {
		var r Message
		r, err = h.node.Send(proto, to, message)
		if err != nil {
			return
		}
		if r.Type == ERROR_RESPONSE {
			err = fmt.Errorf("response error: %v", r.Body)
		} else {
			response = r.Body
		}
	}
	return
}

// Send delivers a message to a node via the given protocol
func (node *Node) Send(proto protocol.ID, addr peer.ID, m *Message) (response Message, err error) {
	s, err := node.Host.NewStream(context.Background(), addr, proto)
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
