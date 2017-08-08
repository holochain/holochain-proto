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
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"gopkg.in/mgo.v2/bson"
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
	DEL_REQUEST
	MOD_REQUEST
	GET_REQUEST
	LINK_REQUEST
	GETLINK_REQUEST
	DELETELINK_REQUEST

	// Gossip messages

	GOSSIP_REQUEST

	// Validate Messages

	VALIDATE_PUT_REQUEST
	VALIDATE_LINK_REQUEST
	VALIDATE_DEL_REQUEST
	VALIDATE_MOD_REQUEST

	// Application Messages

	APP_MESSAGE

	// Peer messages

	LISTADD_REQUEST
)

var ErrBlockedListed = errors.New("node blockedlisted")

// Message represents data that can be sent to node in the network
type Message struct {
	Type MsgType
	Time time.Time
	From peer.ID
	Body interface{}
}

// Node represents a node in the network
type Node struct {
	HashAddr    peer.ID
	NetAddr     ma.Multiaddr
	Host        *rhost.RoutedHost
	mdnsSvc     discovery.Service
	blockedlist map[peer.ID]bool
	protocols   [_protocolCount]*Protocol
}

// Protocol encapsulates data for our different protocols
type Protocol struct {
	ID       protocol.ID
	Receiver ReceiverFn
}

const (
	ActionProtocol = iota
	ValidateProtocol
	GossipProtocol
	_protocolCount
)

type Router struct {
	dummy int
}

func (r *Router) FindPeer(context.Context, peer.ID) (peer pstore.PeerInfo, err error) {
	err = errors.New("routing not implemented")
	return
}

// implement peer found function for mdns discovery
func (h *Holochain) HandlePeerFound(pi pstore.PeerInfo) {
	h.dht.dlog.Logf("discovered peer via mdns: %v", pi)
	if h.node.IsBlocked(pi.ID) {
		h.dht.dlog.Logf("peer %v in blockedlist, ignoring", pi.ID)
	} else {
		h.node.Host.Connect(context.Background(), pi)
		err := h.dht.UpdateGossiper(pi.ID, 0)
		if err != nil {
			h.dht.dlog.Logf("error when updating gossiper: %v", pi)
		}
	}
}

func (n *Node) EnableMDNSDiscovery(notifee discovery.Notifee, interval time.Duration) (err error) {
	ctx := context.Background()

	n.mdnsSvc, err = discovery.NewMdnsService(ctx, n.Host, interval)
	if err != nil {
		return
	}

	n.mdnsSvc.RegisterNotifee(notifee)
	return
}

// NewNode creates a new ipfs basichost node with given identity
func NewNode(listenAddr string, protoMux string, agent *LibP2PAgent) (node *Node, err error) {

	nodeID, _, err := agent.NodeID()
	if err != nil {
		return
	}

	var n Node
	n.NetAddr, err = ma.NewMultiaddr(listenAddr)
	if err != nil {
		return
	}

	ps := pstore.NewPeerstore()

	n.HashAddr = nodeID
	priv := agent.PrivKey()
	ps.AddPrivKey(nodeID, priv)
	ps.AddPubKey(nodeID, priv.GetPublic())

	n.protocols[ValidateProtocol] = &Protocol{protocol.ID("/hc-validate-" + protoMux + "/0.0.0"), ValidateReceiver}
	n.protocols[GossipProtocol] = &Protocol{protocol.ID("/hc-gossip-" + protoMux + "/0.0.0"), GossipReceiver}
	n.protocols[ActionProtocol] = &Protocol{protocol.ID("/hc-action-" + protoMux + "/0.0.0"), ActionReceiver}

	ctx := context.Background()

	// create a new swarm to be used by the service host
	netw, err := swarm.NewNetwork(ctx, []ma.Multiaddr{n.NetAddr}, nodeID, ps, nil)
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

// Fingerprint creates a hash of a message
func (m *Message) Fingerprint() (f Hash, err error) {
	var data []byte
	if m != nil {
		data, err = bson.Marshal(m)

		if err != nil {
			return
		}
		f.H, err = mh.Sum(data, mh.SHA2_256, -1)
	} else {
		f = NullHash()
	}

	return
}

// String converts a message to a nice string
func (m Message) String() string {
	var typeStr string
	switch m.Type {
	case ERROR_RESPONSE:
		typeStr = "ERROR_RESPONSE"
	case OK_RESPONSE:
		typeStr = "OK_RESPONSE"
	case PUT_REQUEST:
		typeStr = "PUT_REQUEST"
	case DEL_REQUEST:
		typeStr = "DEL_REQUEST"
	case MOD_REQUEST:
		typeStr = "MOD_REQUEST"
	case GET_REQUEST:
		typeStr = "GET_REQUEST"
	case LINK_REQUEST:
		typeStr = "LINK_REQUEST"
	case GETLINK_REQUEST:
		typeStr = "GETLINK_REQUEST"
	case DELETELINK_REQUEST:
		typeStr = "DELETELINK_REQUEST"
	case GOSSIP_REQUEST:
		typeStr = "GOSSIP_REQUEST"
	case VALIDATE_PUT_REQUEST:
		typeStr = "VALIDATE_PUT_REQUEST"
	case VALIDATE_LINK_REQUEST:
		typeStr = "VALIDATE_LINK_REQUEST"
	case VALIDATE_DEL_REQUEST:
		typeStr = "VALIDATE_DEL_REQUEST"
	case VALIDATE_MOD_REQUEST:
		typeStr = "VALIDATE_MOD_REQUEST"
	case APP_MESSAGE:
		typeStr = "APP_MESSAGE"
	case LISTADD_REQUEST:
		typeStr = "LISTADD_REQUEST"
	}
	return fmt.Sprintf("%s @ %v From:%v Body:%v", typeStr, m.Time, m.From, m.Body)
}

// respondWith writes a message either error or otherwise, to the stream
func (node *Node) respondWith(s net.Stream, err error, body interface{}) {
	var m *Message
	if err != nil {
		errResp := NewErrorResponse(err)
		errResp.Payload = body
		m = node.NewMessage(ERROR_RESPONSE, errResp)
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
func (node *Node) StartProtocol(h *Holochain, proto int) (err error) {
	node.Host.SetStreamHandler(node.protocols[proto].ID, func(s net.Stream) {
		var m Message
		err := m.Decode(s)
		var response interface{}
		if m.From == "" {
			// @todo other sanity checks on From?
			err = errors.New("message must have a source")
		} else {
			if node.IsBlocked(s.Conn().RemotePeer()) {
				err = ErrBlockedListed
			}

			if err == nil {
				response, err = node.protocols[proto].Receiver(h, &m)
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
func (node *Node) Send(proto int, addr peer.ID, m *Message) (response Message, err error) {

	if node.IsBlocked(addr) {
		err = ErrBlockedListed
		return
	}

	s, err := node.Host.NewStream(context.Background(), addr, node.protocols[proto].ID)
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

// IsBlockedListed checks to see if a node is on the blockedlist
func (node *Node) IsBlocked(addr peer.ID) (ok bool) {
	ok = node.blockedlist[addr]
	return
}

// InitBlockedList sets up the blockedlist from a PeerList
func (node *Node) InitBlockedList(list PeerList) {
	node.blockedlist = make(map[peer.ID]bool)
	for _, r := range list.Records {
		node.Block(r.ID)
	}
}

// Block adds a peer to the blocklist
func (node *Node) Block(addr peer.ID) {
	if node.blockedlist == nil {
		node.blockedlist = make(map[peer.ID]bool)
	}
	node.blockedlist[addr] = true
}

// Unblock removes a peer from the blocklist
func (node *Node) Unblock(addr peer.ID) {
	if node.blockedlist != nil {
		delete(node.blockedlist, addr)
	}
}

type ErrorResponse struct {
	Code    int
	Message string
	Payload interface{}
}

const (
	ErrUnknownCode = iota
	ErrHashNotFoundCode
	ErrHashDeletedCode
	ErrHashModifiedCode
	ErrHashRejectedCode
	ErrLinkNotFoundCode
	ErrEntryTypeMismatchCode
	ErrBlockedListedCode
)

// NewErrorResponse encodes standard errors for transmitting
func NewErrorResponse(err error) (errResp ErrorResponse) {
	switch err {
	case ErrHashNotFound:
		errResp.Code = ErrHashNotFoundCode
	case ErrHashDeleted:
		errResp.Code = ErrHashDeletedCode
	case ErrHashModified:
		errResp.Code = ErrHashModifiedCode
	case ErrHashRejected:
		errResp.Code = ErrHashRejectedCode
	case ErrLinkNotFound:
		errResp.Code = ErrLinkNotFoundCode
	case ErrEntryTypeMismatch:
		errResp.Code = ErrEntryTypeMismatchCode
	case ErrBlockedListed:
		errResp.Code = ErrBlockedListedCode
	default:
		errResp.Message = err.Error() //Code will be set to ErrUnknown by default cus it's 0
	}
	return
}

// DecodeResponseError creates a go error object from the ErrorResponse data
func (errResp ErrorResponse) DecodeResponseError() (err error) {
	switch errResp.Code {
	case ErrHashNotFoundCode:
		err = ErrHashNotFound
	case ErrHashDeletedCode:
		err = ErrHashDeleted
	case ErrHashModifiedCode:
		err = ErrHashModified
	case ErrHashRejectedCode:
		err = ErrHashRejected
	case ErrLinkNotFoundCode:
		err = ErrLinkNotFound
	case ErrEntryTypeMismatchCode:
		err = ErrEntryTypeMismatch
	case ErrBlockedListedCode:
		err = ErrBlockedListed
	default:
		err = errors.New(errResp.Message)
	}
	return
}
