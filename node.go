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

	goprocess "github.com/jbenet/goprocess"
	goprocessctx "github.com/jbenet/goprocess/context"
	nat "github.com/libp2p/go-libp2p-nat"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	. "github.com/metacurrency/holochain/hash"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	"gopkg.in/mgo.v2/bson"
	"io"
	"math/big"
	"math/rand"
	go_net "net"
	"strconv"
	"strings"
	"sync"
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

	// Kademlia messages

	FIND_NODE_REQUEST
)

func (msgType MsgType) String() string {
	return []string{"ERROR_RESPONSE",
		"OK_RESPONSE",
		"PUT_REQUEST",
		"DEL_REQUEST",
		"MOD_REQUEST",
		"GET_REQUEST",
		"LINK_REQUEST",
		"GETLINK_REQUEST",
		"DELETELINK_REQUEST",
		"GOSSIP_REQUEST",
		"VALIDATE_PUT_REQUEST",
		"VALIDATE_LINK_REQUEST",
		"VALIDATE_DEL_REQUEST",
		"VALIDATE_MOD_REQUEST",
		"APP_MESSAGE",
		"LISTADD_REQUEST",
		"FIND_NODE_REQUEST"}[msgType]
}

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
	HashAddr     peer.ID
	NetAddr      ma.Multiaddr
	host         *rhost.RoutedHost
	mdnsSvc      discovery.Service
	blockedlist  map[peer.ID]bool
	protocols    [_protocolCount]*Protocol
	peerstore    pstore.Peerstore
	routingTable *RoutingTable
	nat          *nat.NAT
	log          *Logger

	// ticker task stoppers
	retrying      chan bool
	gossiping     chan bool
	bootstrapping chan bool
	refreshing    chan bool

	// items for the kademlia implementation
	plk   sync.Mutex
	peers map[peer.ID]*peerTracker
	ctx   context.Context
	proc  goprocess.Process
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
	KademliaProtocol
	_protocolCount
)

const (
	PeerTTL                       = time.Minute * 10
	DefaultRoutingRefreshInterval = time.Minute
	DefaultGossipInterval         = time.Second * 2
)

// implement peer found function for mdns discovery
func (h *Holochain) HandlePeerFound(pi pstore.PeerInfo) {
	if h.dht != nil {
		h.dht.dlog.Logf("discovered peer via mdns: %v", pi)
		err := h.AddPeer(pi)
		if err != nil {
			h.dht.dlog.Logf("error when adding peer: %v, %v", pi, err)
		}
	}
}

func (h *Holochain) addPeer(pi pstore.PeerInfo, confirm bool) (err error) {
	// add the peer into the peerstore
	h.node.peerstore.AddAddrs(pi.ID, pi.Addrs, PeerTTL)

	// attempt a connection to see if this is actually valid
	if confirm {
		err = h.node.host.Connect(h.node.ctx, pi)
	}
	if err != nil {
		h.dht.dlog.Logf("Clearing peer %v, connection failed (%v)\n", pi.ID, err)
		h.node.peerstore.ClearAddrs(pi.ID)
		err = nil
	} else {
		bootstrap := h.node.routingTable.IsEmpty()
		h.dht.dlog.Logf("Adding Peer: %v\n", pi.ID)
		h.node.routingTable.Update(pi.ID)
		err = h.dht.AddGossiper(pi.ID)
		if bootstrap {
			RoutingRefreshTask(h)
		}
	}
	return
}

// RoutingRefreshTask fills the routing table by searching for a random node
func RoutingRefreshTask(h *Holochain) {
	s := fmt.Sprintf("%d", rand.Intn(1000000))
	var hash Hash
	err := hash.Sum(h.hashSpec, []byte(s))
	if err == nil {
		h.node.FindPeer(h.node.ctx, PeerIDFromHash(hash))
	}
}

func (node *Node) isPeerActive(id peer.ID) bool {
	// inactive currently defined by seeing if there are any addrs in the
	// peerstore
	// TODO: should be something different
	addrs := node.peerstore.Addrs(id)
	return len(addrs) > 0
}

// filterInactviePeers removes peers from a list who are currently inactive
// if max >  0 returns only max number of peers
func (node *Node) filterInactviePeers(peersIn []peer.ID, max int) (peersOut []peer.ID) {
	if max <= 0 {
		max = len(peersIn)
	}
	var i int
	for _, p := range peersIn {
		if node.isPeerActive(p) {
			peersOut = append(peersOut, p)
			i += 1
			if i == max {
				return
			}
		}
	}
	return
}

// AddPeer adds a peer to the peerstore if it passes various checks
func (h *Holochain) AddPeer(pi pstore.PeerInfo) (err error) {
	h.dht.dlog.Logf("Adding Peer Req: %v my node %v\n", pi.ID, h.node.HashAddr)
	if pi.ID == h.node.HashAddr {
		return
	}
	if h.node.IsBlocked(pi.ID) {
		err = ErrBlockedListed
	} else {
		err = h.addPeer(pi, true)
	}
	return
}

func (n *Node) EnableMDNSDiscovery(h *Holochain, interval time.Duration) (err error) {
	ctx := context.Background()
	tag := h.dnaHash.String() + "._udp"
	n.mdnsSvc, err = discovery.NewMdnsService(ctx, n.host, interval, tag)
	if err != nil {
		return
	}
	n.mdnsSvc.RegisterNotifee(h)
	return
}

func (n *Node) ExternalAddr() ma.Multiaddr {
	if n.nat == nil {
		return n.NetAddr
	} else {
		mappings := n.nat.Mappings()
		for i := 0; i < len(mappings); i++ {
			external_addr, err := mappings[i].ExternalAddr()
			if err == nil {
				return external_addr
			}
		}
		return n.NetAddr
	}
}

func (node *Node) discoverAndHandleNat(listenPort int) {
	node.log.Logf("Looking for a NAT...")
	node.nat = nat.DiscoverNAT()
	if node.nat == nil {
		node.log.Logf("No NAT found.")
	} else {
		node.log.Logf("Discovered NAT! Trying to aquire public port mapping via UPnP...")
		ifaces, _ := go_net.Interfaces()
		// handle err
		for _, i := range ifaces {
			addrs, _ := i.Addrs()
			// handle err
			for _, addr := range addrs {
				var ip go_net.IP
				switch v := addr.(type) {
				case *go_net.IPNet:
					ip = v.IP
				case *go_net.IPAddr:
					ip = v.IP
				}
				if ip.Equal(go_net.IPv4(127, 0, 0, 1)) {
					continue
				}
				addr_string := fmt.Sprintf("/ip4/%s/tcp/%d", ip, listenPort)
				localaddr, err := ma.NewMultiaddr(addr_string)
				if err == nil {
					node.log.Logf("NAT: trying to establish NAT mapping for %s...", addr_string)
					node.nat.NewMapping(localaddr)
				}
			}
		}

		external_addr := node.ExternalAddr()

		if external_addr != node.NetAddr {
			node.log.Logf("NAT: successfully created port mapping! External address is: %s", external_addr.String())
		} else {
			node.log.Logf("NAT: could not create port mappping. Keep trying...")
			Infof("NAT:-------------------------------------------------------")
			Infof("NAT:---------------------Warning---------------------------")
			Infof("NAT:-------------------------------------------------------")
			Infof("NAT: You seem to be behind a NAT that does not speak UPnP.")
			Infof("NAT: You will have to setup a port forwarding manually.")
			Infof("NAT: This instance is configured to listen on port: %d", listenPort)
			Infof("NAT:-------------------------------------------------------")
		}

	}
}

// NewNode creates a new node with given multiAddress listener string and identity
func NewNode(listenAddr string, protoMux string, agent *LibP2PAgent, enableNATUPnP bool, log *Logger) (node *Node, err error) {
	var n Node
	n.log = log
	n.log.Logf("Creating new node with protoMux: %s\n", protoMux)
	nodeID, _, err := agent.NodeID()
	if err != nil {
		return
	}
	n.log.Logf("NodeID is: %v\n", nodeID)

	listenPort, err := strconv.Atoi(strings.Split(listenAddr, "/")[4])
	if err != nil {
		Infof("Can't parse port from Multiaddress string: %s", listenAddr)
		return
	}

	n.NetAddr, err = ma.NewMultiaddr(listenAddr)
	if err != nil {
		return
	}

	if enableNATUPnP {
		n.discoverAndHandleNat(listenPort)
	}

	ps := pstore.NewPeerstore()
	n.peerstore = ps
	ps.AddAddrs(nodeID, []ma.Multiaddr{n.NetAddr}, pstore.PermanentAddrTTL)

	n.HashAddr = nodeID
	priv := agent.PrivKey()
	ps.AddPrivKey(nodeID, priv)
	ps.AddPubKey(nodeID, priv.GetPublic())

	validateProtocolString := "/hc-validate-" + protoMux + "/0.0.0"
	gossipProtocolString := "/hc-gossip-" + protoMux + "/0.0.0"
	actionProtocolString := "/hc-action-" + protoMux + "/0.0.0"
	kademliaProtocolString := "/hc-kademlia-" + protoMux + "/0.0.0"

	n.log.Logf("Validate protocol identifier: " + validateProtocolString)
	n.log.Logf("Gossip protocol identifier: " + gossipProtocolString)
	n.log.Logf("Action protocol identifier: " + actionProtocolString)
	n.log.Logf("Kademlia protocol identifier: " + kademliaProtocolString)

	n.protocols[ValidateProtocol] = &Protocol{protocol.ID(validateProtocolString), ValidateReceiver}
	n.protocols[GossipProtocol] = &Protocol{protocol.ID(gossipProtocolString), GossipReceiver}
	n.protocols[ActionProtocol] = &Protocol{protocol.ID(actionProtocolString), ActionReceiver}
	n.protocols[KademliaProtocol] = &Protocol{protocol.ID(kademliaProtocolString), KademliaReceiver}

	ctx := context.Background()
	n.ctx = ctx

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

	n.host = rhost.Wrap(bh, &n)

	m := pstore.NewMetrics()
	n.routingTable = NewRoutingTable(KValue, nodeID, time.Minute, m)
	n.peers = make(map[peer.ID]*peerTracker)

	node = &n

	n.host.Network().Notify((*netNotifiee)(node))

	n.proc = goprocessctx.WithContextAndTeardown(ctx, func() error {
		// remove ourselves from network notifs.
		n.host.Network().StopNotify((*netNotifiee)(node))
		return n.host.Close()
	})

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
	return fmt.Sprintf("%v @ %v From:%v Body:%v", m.Type, m.Time, m.From, m.Body)
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
		Infof("Response failed: unable to encode message: %v", m)
	}
	_, err = s.Write(data)
	if err != nil {
		Infof("Response failed: write returned error: %v", err)
	}
}

// StartProtocol initiates listening for a protocol on the node
func (node *Node) StartProtocol(h *Holochain, proto int) (err error) {
	node.host.SetStreamHandler(node.protocols[proto].ID, func(s net.Stream) {
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
	if node.gossiping != nil {
		node.log.Log("Stopping gossiping")
		stop := node.gossiping
		node.gossiping = nil
		stop <- true
	}
	if node.retrying != nil {
		node.log.Log("Stopping retrying")
		stop := node.retrying
		node.retrying = nil
		stop <- true
	}
	if node.bootstrapping != nil {
		node.log.Log("Stopping boostrapping")
		stop := node.bootstrapping
		node.bootstrapping = nil
		stop <- true
	}
	return node.proc.Close()
}

// Send delivers a message to a node via the given protocol
func (node *Node) Send(ctx context.Context, proto int, addr peer.ID, m *Message) (response Message, err error) {

	if node.IsBlocked(addr) {
		err = ErrBlockedListed
		return
	}

	s, err := node.host.NewStream(ctx, addr, node.protocols[proto].ID)
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
		node.log.Logf("failed to decode with err:%v ", err)
		return
	}
	return
}

// NewMessage creates a message from the node with a new current timestamp
func (node *Node) NewMessage(t MsgType, body interface{}) (msg *Message) {
	m := Message{Type: t, Time: time.Now().Round(0), Body: body, From: node.HashAddr}
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

// Distance returns the nodes peer distance to another node for purposes of gossip
func (node *Node) Distance(id peer.ID) *big.Int {
	h := HashFromPeerID(id)
	nh := HashFromPeerID(node.HashAddr)
	return HashXORDistance(nh, h)
}

// Context return node's context
func (node *Node) Context() context.Context {
	return node.ctx
}

// Process return node's process
func (node *Node) Process() goprocess.Process {
	return node.proc
}
