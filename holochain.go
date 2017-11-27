// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------

// Data integrity engine for distributed applications -- a validating monotonic
// DHT "backed" by authoritative hashchains for data provenance.
package holochain

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	mh "github.com/multiformats/go-multihash"
	"github.com/tidwall/buntdb"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// Version is the numeric version number of the holochain library
	Version int = 18

	// VersionStr is the textual version number of the holochain library
	VersionStr string = "18"

	// DefaultSendTimeout a time.Duration to wait by default for send to complete
	DefaultSendTimeout = 3000 * time.Millisecond
)

// Loggers holds the logging structures for the different parts of the system
type Loggers struct {
	App        Logger
	Debug      Logger
	DHT        Logger
	Gossip     Logger
	TestPassed Logger
	TestFailed Logger
	TestInfo   Logger
}

// Config holds the non-DNA configuration for a holo-chain, from config file or environment variables
type Config struct {
	Port            int
	EnableMDNS      bool
	PeerModeAuthor  bool
	PeerModeDHTNode bool
	EnableNATUPnP   bool
	BootstrapServer string
	Loggers         Loggers

	gossipInterval           time.Duration
	bootstrapRefreshInterval time.Duration
	routingRefreshInterval   time.Duration
	retryInterval            time.Duration
}

// Progenitor holds data on the creator of the DNA
type Progenitor struct {
	Identity string
	PubKey   []byte
}

// Holochain struct holds the full "DNA" of the holochain (all your app code for managing distributed data integrity)
type Holochain struct {
	//---- lowercase private values not serialized; initialized on Load
	nodeID           peer.ID // this is hash of the public key of the id and acts as the node address
	nodeIDStr        string  // this is just a cached version of the nodeID B58 string encoded
	dnaHash          Hash
	agentHash        Hash
	agentTopHash     Hash
	rootPath         string
	agent            Agent
	encodingFormat   string
	hashSpec         HashSpec
	Config           Config
	dht              *DHT
	nucleus          *Nucleus
	node             *Node
	chain            *Chain // This node's local source chain
	bridgeDB         *buntdb.DB
	validateProtocol *Protocol
	gossipProtocol   *Protocol
	actionProtocol   *Protocol
	asyncSends       chan error
}

func (h *Holochain) Nucleus() (n *Nucleus) {
	return h.nucleus
}

func (h *Holochain) Chain() (n *Chain) {
	return h.chain
}

func (h *Holochain) Name() string {
	return h.nucleus.dna.Name
}

var debugLog Logger
var infoLog Logger
var SendTimeoutErr = errors.New("send timeout")

// Debug sends a string to the standard debug log
func Debug(m string) {
	debugLog.Log(m)
}

func (h *Holochain) Debug(m string) {
	h.Config.Loggers.Debug.Log(m)
}

// Debugf sends a formatted string to the debug log
func (h *Holochain) Debugf(m string, args ...interface{}) {
	h.Config.Loggers.Debug.Logf(m, args...)
}

// Debugf sends a formatted string to the global debug log
func Debugf(m string, args ...interface{}) {
	debugLog.Logf(m, args...)
}

// Info sends a string to the global info log
func Info(m string) {
	infoLog.Log(m)
}

// Infof sends a formatted string to the standard info log
func Infof(m string, args ...interface{}) {
	infoLog.Logf(m, args...)
}

// DebuggingRequestedViaEnv determines whether an environment var was set to enable or disable debugging
func DebuggingRequestedViaEnv() (val, yes bool) {
	return envBoolRequest("HCDEBUG")
}

func envBoolRequest(env string) (val, yes bool) {
	str := strings.ToLower(os.Getenv(env))
	yes = str != ""
	if yes {
		val = str == "true" || str == "1"
	}
	return
}

var _holochainInitialized bool

// InitializeHolochain setup function that must be called once at startup
// by the application that uses this holochain library
func InitializeHolochain() {
	// this should only run once
	if !_holochainInitialized {
		gob.Register(Header{})
		gob.Register(AgentEntry{})
		gob.Register(Hash{})
		gob.Register(PutReq{})
		gob.Register(GetReq{})
		gob.Register(GetResp{})
		gob.Register(ModReq{})
		gob.Register(DelReq{})
		gob.Register(LinkReq{})
		gob.Register(LinkQuery{})
		gob.Register(GossipReq{})
		gob.Register(Gossip{})
		gob.Register(ValidateQuery{})
		gob.Register(ValidateResponse{})
		gob.Register(Put{})
		gob.Register(GobEntry{})
		gob.Register(LinkQueryResp{})
		gob.Register(TaggedHash{})
		gob.Register(ErrorResponse{})
		gob.Register(DelEntry{})
		gob.Register(StatusChange{})
		gob.Register(Package{})
		gob.Register(AppMsg{})
		gob.Register(ListAddReq{})
		gob.Register(FindNodeReq{})
		gob.Register(CloserPeersResp{})
		gob.Register(PeerInfo{})

		RegisterBultinRibosomes()

		infoLog.New(nil)
		infoLog.Enabled = true
		debugLog.Format = "HC: %{file}.%{line}: %{message}"
		val, yes := DebuggingRequestedViaEnv()
		if yes {
			debugLog.Enabled = val
		}
		debugLog.New(nil)

		rand.Seed(time.Now().Unix()) // initialize global pseudo random generator

		_holochainInitialized = true
	}
}

// ZomePath returns the path to the zome dna data
// @todo sanitize the name value
func (h *Holochain) ZomePath(z *Zome) string {
	return filepath.Join(h.DNAPath(), z.Name)
}

// NewHolochain creates a new holochain structure with a randomly generated ID and default values
func NewHolochain(agent Agent, root string, format string, zomes ...Zome) Holochain {
	u, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	pk, err := agent.PubKey().Bytes()
	if err != nil {
		panic(err)
	}

	dna := DNA{
		UUID:            u,
		RequiresVersion: Version,
		DHTConfig:       DHTConfig{HashType: "sha2-256"},
		Progenitor:      Progenitor{Identity: string(agent.Identity()), PubKey: pk},
		Zomes:           zomes,
	}

	h := Holochain{
		agent:          agent,
		rootPath:       root,
		encodingFormat: format,
	}

	h.nucleus = NewNucleus(&h, &dna)

	// once the agent is set up we can calculate the id
	h.nodeID, h.nodeIDStr, err = agent.NodeID()
	if err != nil {
		panic(err)
	}

	h.PrepareHashType()

	return h
}

// Agent exposes the agent element
func (h *Holochain) Agent() Agent {
	return h.agent
}

// NodeIDStr exposes the agent element
func (h *Holochain) NodeIDStr() string {
	return h.nodeIDStr
}

// PrepareHashType makes sure the given string is a correct multi-hash and stores
// the code and length to the Holochain struct
func (h *Holochain) PrepareHashType() (err error) {
	c, ok := mh.Names[h.nucleus.dna.DHTConfig.HashType]
	if !ok {
		return fmt.Errorf("Unknown hash type: %s", h.nucleus.dna.DHTConfig.HashType)
	}
	h.hashSpec.Code = c
	h.hashSpec.Length = -1
	return
}

// createNode creates a network node based on the current agent and port data
func (h *Holochain) createNode() (err error) {
	var ip string
	if os.Getenv("_HCTEST") == "1" {
		ip = "127.0.0.1"
	} else {
		ip = "0.0.0.0"
	}
	listenaddr := fmt.Sprintf("/ip4/%s/tcp/%d", ip, h.Config.Port)
	h.node, err = NewNode(listenaddr, h.dnaHash.String(), h.Agent().(*LibP2PAgent), h.Config.EnableNATUPnP, &h.Config.Loggers.Debug)
	return
}

// Prepare sets up a holochain to run by:
// loading the schema validators, setting up a Network node and setting up the DHT
func (h *Holochain) Prepare() (err error) {
	h.Debugf("Preparing %v", h.dnaHash)

	err = h.nucleus.dna.check()
	if err != nil {
		return
	}

	if err = h.PrepareHashType(); err != nil {
		return
	}

	h.asyncSends = make(chan error, 10)

	err = h.createNode()
	if err != nil {
		return
	}

	h.dht = NewDHT(h)
	h.nucleus.h = h

	var peerList PeerList
	peerList, err = h.dht.getList(BlockedList)
	if err != nil {
		return err
	}

	h.node.InitBlockedList(peerList)
	return
}

// Activate fires up the holochain node, starting node discovery and protocols
func (h *Holochain) Activate() (err error) {
	h.Debugf("Activating  %v", h.dnaHash)

	if h.Config.EnableMDNS {
		err = h.node.EnableMDNSDiscovery(h, time.Second)
		if err != nil {
			return
		}
	}
	if h.Config.PeerModeDHTNode {
		if err = h.dht.Start(); err != nil {
			return
		}

	}
	if h.Config.PeerModeAuthor {
		if err = h.nucleus.Start(); err != nil {
			return
		}
	}
	return
}

// RootPath returns a holochain root path
func (h *Holochain) RootPath() string {
	return h.rootPath
}

// UIPath returns a holochain UI path
func (h *Holochain) UIPath() string {
	return filepath.Join(h.rootPath, ChainUIDir)
}

// DBPath returns a holochain DB path
func (h *Holochain) DBPath() string {
	return filepath.Join(h.rootPath, ChainDataDir)
}

// DNAPath returns a holochain DNA path
func (h *Holochain) DNAPath() string {
	return filepath.Join(h.rootPath, ChainDNADir)
}

// TestPath returns the path to a holochain's test directory
func (h *Holochain) TestPath() string {
	return filepath.Join(h.rootPath, ChainTestDir)
}

// DNAHash returns the hash of the DNA entry which is also the holochain ID
func (h *Holochain) DNAHash() (id Hash) {
	return h.dnaHash.Clone()
}

// AgentHash returns the hash of the Agent entry
func (h *Holochain) AgentHash() (id Hash) {
	return h.agentHash.Clone()
}

// AgentHash returns the hash of the Agent entry
func (h *Holochain) AgentTopHash() (id Hash) {
	return h.agentTopHash.Clone()
}

// Top returns a hash of top header or err if not yet defined
func (h *Holochain) Top() (top Hash, err error) {
	//TODO: LOCK!!!
	tp := h.chain.Hashes[len(h.chain.Hashes)-1]
	top = tp.Clone()
	return
}

// Started returns true if the chain has been gened
func (h *Holochain) Started() bool {
	return h.DNAHash().String() != ""
}

// AddAgentEntry adds a new sys entry type setting the current agent data (identity and key)
func (h *Holochain) AddAgentEntry(revocation Revocation) (headerHash, agentHash Hash, err error) {
	var entry AgentEntry

	entry, err = h.agent.AgentEntry(revocation)
	if err != nil {
		return
	}
	e := GobEntry{C: entry}

	var agentHeader *Header
	headerHash, agentHeader, err = h.NewEntry(time.Now(), AgentEntryType, &e)
	if err != nil {
		return
	}
	agentHash = agentHeader.EntryLink
	return
}

// GenChain establishes a holochain instance by creating the initial genesis entries in the chain
// It assumes a properly set up .holochain sub-directory with a config file and keys for signing.
func (h *Holochain) GenChain() (headerHash Hash, err error) {

	if h.Started() {
		err = mkErr("chain already started")
		return
	}

	defer func() {
		if err != nil {
			err = fmt.Errorf("Error during chain genesis: %v\n", err)
			os.RemoveAll(h.rootPath)
		}
	}()

	var buf bytes.Buffer
	err = h.EncodeDNA(&buf)

	e := GobEntry{C: buf.Bytes()}

	var dnaHeader *Header
	_, dnaHeader, err = h.NewEntry(time.Now(), DNAEntryType, &e)
	if err != nil {
		return
	}

	h.dnaHash = dnaHeader.EntryLink.Clone()

	var agentHash Hash
	headerHash, agentHash, err = h.AddAgentEntry(nil) // revocation is empty on initial Gen
	if err != nil {
		return
	}

	h.agentHash = agentHash
	h.agentTopHash = agentHash

	if err = WriteFile([]byte(h.dnaHash.String()), h.rootPath, DNAHashFileName); err != nil {
		return
	}

	if err = h.Prepare(); err != nil {
		return
	}

	err = h.dht.SetupDHT()
	if err != nil {
		return
	}

	err = h.nucleus.RunGenesis()
	if err != nil {
		return
	}

	return
}

func initLogger(l *Logger, envOverride string, writer io.Writer) (err error) {
	if err = l.New(writer); err != nil {
		return
	}
	d := os.Getenv(envOverride)
	switch d {
	case "true":
		fallthrough
	case "TRUE":
		fallthrough
	case "1":
		Debugf("Using environment variable (%s) to enable log", envOverride)
		l.Enabled = true
	case "false":
		fallthrough
	case "FALSE":
		fallthrough
	case "0":
		Debugf("Using environment variable (%s) to disable log", envOverride)
		l.Enabled = false
	}
	return
}

func (config *Config) Setup() (err error) {
	config.gossipInterval = DefaultGossipInterval
	config.bootstrapRefreshInterval = BootstrapTTL
	config.routingRefreshInterval = DefaultRoutingRefreshInterval
	config.retryInterval = DefaultRetryInterval
	err = config.SetupLogging()
	return
}

func (config *Config) SetGossipInterval(interval time.Duration) {
	config.gossipInterval = interval
}

// SetupLogging initializes loggers as configured by the config file and environment variables
func (config *Config) SetupLogging() (err error) {
	if err = initLogger(&config.Loggers.Debug, "HCLOG_DEBUG_ENABLE", nil); err != nil {
		return
	}
	if err = initLogger(&config.Loggers.App, "HCLOG_APP_ENABLE", nil); err != nil {
		return
	}
	if err = initLogger(&config.Loggers.DHT, "HCLOG_DHT_ENABLE", nil); err != nil {
		return
	}
	if err = initLogger(&config.Loggers.Gossip, "HCLOG_GOSSIP_ENABLE", nil); err != nil {
		return
	}
	if err = config.Loggers.TestPassed.New(nil); err != nil {
		return
	}
	if err = config.Loggers.TestFailed.New(os.Stderr); err != nil {
		return
	}
	if err = config.Loggers.TestInfo.New(nil); err != nil {
		return
	}
	val := os.Getenv("HCLOG_PREFIX")
	if val != "" {
		Debugf("Using environment variable to set log prefix to: %s", val)
		config.Loggers.Debug.SetPrefix(val)
		config.Loggers.App.SetPrefix(val)
		config.Loggers.DHT.SetPrefix(val)
		config.Loggers.Gossip.SetPrefix(val)
		config.Loggers.TestPassed.SetPrefix(val)
		config.Loggers.TestFailed.SetPrefix(val)
		config.Loggers.TestInfo.SetPrefix(val)
		//		debugLog.SetPrefix(val)
		//		infoLog.SetPrefix(val)
	}
	return
}

// EncodeDNA encodes a holochain's DNA to an io.Writer
func (h *Holochain) EncodeDNA(writer io.Writer) (err error) {
	return Encode(writer, h.encodingFormat, &h.nucleus.dna)
}

// NewEntry adds an entry and it's header to the chain and returns the header and it's hash
func (h *Holochain) NewEntry(now time.Time, entryType string, entry Entry) (hash Hash, header *Header, err error) {
	h.chain.lk.Lock()
	defer h.chain.lk.Unlock()
	var l int
	l, hash, header, err = h.chain.prepareHeader(now, entryType, entry, h.agent.PrivKey(), nil)
	if err == nil {
		err = h.chain.addEntry(l, hash, header, entry)
	}

	if err == nil {
		var e interface{} = entry
		if entryType == DNAEntryType {
			e = "<DNA>"
		}
		h.Debugf("NewEntry of %s added as: %s (entry: %v)", entryType, header.EntryLink, e)
	} else {
		h.Debugf("NewEntry of %s failed with: %s (entry: %v)", entryType, err, entry)
	}

	return
}

// Walk takes the argument fn which must be WalkerFn
// Every WalkerFn is of the form:
// func(key *Hash, h *Header, entry interface{}) error
func (h *Holochain) Walk(fn WalkerFn, entriesToo bool) (err error) {
	err = h.chain.Walk(fn)
	return
}

// GetEntryDef returns an EntryDef of the given name
// @TODO this makes the incorrect assumption that entry type strings are unique across zomes
func (h *Holochain) GetEntryDef(t string) (zome *Zome, d *EntryDef, err error) {
	if t == DNAEntryType {
		d = DNAEntryDef
		return
	} else if t == AgentEntryType {
		d = AgentEntryDef
		return
	} else if t == KeyEntryType {
		d = KeyEntryDef
		return
	}
	for _, z := range h.nucleus.dna.Zomes {
		d, err = z.GetEntryDef(t)
		if err == nil {
			zome = &z
			return
		}
	}
	return
}

func (h *Holochain) GetPrivateEntryDefs() (privateDefs []EntryDef) {
	privateDefs = make([]EntryDef, 0)
	for _, z := range h.nucleus.dna.Zomes {
		privateDefs = append(privateDefs, z.GetPrivateEntryDefs()...)
	}
	return
}

// Call executes an exposed function
func (h *Holochain) Call(zomeType string, function string, arguments interface{}, exposureContext string) (result interface{}, err error) {
	n, z, err := h.MakeRibosome(zomeType)
	if err != nil {
		return
	}
	fn, err := z.GetFunctionDef(function)
	if err != nil {
		return
	}
	if !fn.ValidExposure(exposureContext) {
		err = errors.New("function not available")
		return
	}
	result, err = n.Call(fn, arguments)
	return
}

// MakeRibosome creates a Ribosome object based on the zome type
func (h *Holochain) MakeRibosome(t string) (r Ribosome, z *Zome, err error) {
	z, err = h.GetZome(t)
	if err != nil {
		return
	}
	r, err = z.MakeRibosome(h)
	return
}

// GetProperty returns the value of a DNA property
func (h *Holochain) GetProperty(prop string) (property string, err error) {
	if prop == ID_PROPERTY || prop == AGENT_ID_PROPERTY || prop == AGENT_NAME_PROPERTY {
		ChangeAppProperty.Log()
	} else {
		property = h.nucleus.dna.Properties[prop]
	}
	return
}

// GetZome returns a zome structure given its name
func (h *Holochain) GetZome(zName string) (z *Zome, err error) {
	for _, zome := range h.nucleus.dna.Zomes {
		if zome.Name == zName {
			z = &zome
			break
		}
	}
	if z == nil {
		err = errors.New("unknown zome: " + zName)
		return
	}
	return
}

// Close releases the resources associated with a holochain
func (h *Holochain) Close() {
	if h.chain.s != nil {
		h.chain.s.Close()
	}
	if h.dht != nil {
		h.dht.Close()
	}
	if h.node != nil {
		h.node.Close()
	}
}

// Reset deletes all chain and dht data and resets data structures
func (h *Holochain) Reset() (err error) {

	h.dnaHash = Hash{}
	h.agentHash = Hash{}
	h.agentTopHash = Hash{}

	if h.chain.s != nil {
		h.chain.s.Close()
	}

	if h.node != nil {
		h.node.Close()
	}

	err = os.RemoveAll(h.DBPath())
	if err != nil {
		return
	}

	if err = os.MkdirAll(h.DBPath(), os.ModePerm); err != nil {
		return
	}
	h.chain, err = NewChainFromFile(h.hashSpec, filepath.Join(h.DBPath(), StoreFileName))
	if err != nil {
		return
	}

	err = os.RemoveAll(filepath.Join(h.rootPath, DNAHashFileName))
	if err != nil {
		panic(err)
	}
	if h.dht != nil {
		h.dht.Close()
	}
	h.dht = NewDHT(h)
	if h.asyncSends != nil {
		close(h.asyncSends)
		h.asyncSends = nil
	}

	return
}

// DHT exposes the DHT structure
func (h *Holochain) DHT() *DHT {
	return h.dht
}

// DHT exposes the Node structure
func (h *Holochain) Node() *Node {
	return h.node
}

// HashSpec exposes the hashSpec structure
func (h *Holochain) HashSpec() HashSpec {
	return h.hashSpec
}

// SendAsync builds a message and either delivers it locally or over the network via node.Send but registers a function for asyncronous call back
func (h *Holochain) SendAsync(proto int, to peer.ID, msg *Message, callback *Callback, timeout time.Duration) (err error) {
	var response interface{}

	go func() {
		response, err = h.Send(h.node.ctx, proto, to, msg, timeout)
		if err == nil {
			var r Ribosome
			r, _, err := h.MakeRibosome(callback.zomeType)
			if err == nil {
				switch t := response.(type) {
				case AppMsg:
					//var result interface{}
					_, err = r.RunAsyncSendResponse(t, callback.Function, callback.ID)

				default:
					err = fmt.Errorf("unimplemented async send response type: %t", t)
				}
			}
		}
		h.asyncSends <- err
	}()
	return
}

// HandleAsyncSends waits on a channel for asyncronous sends
func (h *Holochain) HandleAsyncSends() (err error) {
	for {
		h.Debug("waiting for aysnc send response")
		err, ok := <-h.asyncSends
		if !ok {
			h.Debug("channel closed, breaking")
			break
		}
		h.Debugf("got %v", err)
	}
	return nil
}

const (
	DefaultRetryInterval = time.Millisecond * 500
)

//TaskTicker creates a closure for a holochain task
func (h *Holochain) TaskTicker(interval time.Duration, fn func(h *Holochain)) chan bool {
	if interval > 0 {
		return Ticker(interval, func() { fn(h) })
	}
	return nil
}

// StartBackgroundTasks sets the various background processes in motion
func (h *Holochain) StartBackgroundTasks() {
	go h.DHT().HandleGossipPuts()
	go h.DHT().HandleGossipWiths()
	go h.HandleAsyncSends()

	if h.Config.gossipInterval > 0 {
		h.node.gossiping = h.TaskTicker(h.Config.gossipInterval, GossipTask)
	} else {
		h.Debug("Gossip disabled")
	}
	h.node.retrying = h.TaskTicker(h.Config.retryInterval, RetryTask)
	if h.Config.BootstrapServer != "" {
		BootstrapRefreshTask(h)
		h.node.retrying = h.TaskTicker(h.Config.bootstrapRefreshInterval, BootstrapRefreshTask)
	}
	h.node.refreshing = h.TaskTicker(h.Config.routingRefreshInterval, RoutingRefreshTask)
}

// BootstrapRefreshTask refreshes our node and gets nodes from the bootstrap server
func BootstrapRefreshTask(h *Holochain) {
	e := h.BSpost()
	if e != nil {
		h.dht.dlog.Logf("error in BSpost: %s", e.Error())
	}
	e = h.BSget()
	if e != nil {
		h.dht.dlog.Logf("error in BSget: %s", e.Error())
	}
}

// Send builds a message and either delivers it locally or over the network via node.Send
func (h *Holochain) Send(basectx context.Context, proto int, to peer.ID, message *Message, timeout time.Duration) (response interface{}, err error) {
	f, err := message.Fingerprint()
	if err != nil {
		panic(fmt.Sprintf("error calculating fingerprint when sending message %v", message))
	}
	if timeout == 0 {
		timeout = DefaultSendTimeout
	}
	ctx, cancel := context.WithTimeout(basectx, timeout)
	defer cancel()
	sent := make(chan error, 1)
	go func() {
		// if we are sending to ourselves we should bypass the network mechanics and call
		// the receiver directly
		if to == h.node.HashAddr {
			h.Debugf("Sending message (local):%v (fingerprint:%s)", message, f)
			response, err = h.node.protocols[proto].Receiver(h, message)
			h.Debugf("send result (local): %v (fp:%s)error:%v", response, f, err)
		} else {
			h.Debugf("Sending message to %v (net):%v (fingerprint:%s)", to, message, f)
			var r Message
			r, err = h.node.Send(ctx, proto, to, message)
			h.Debugf("send result to %v (net): %v (fp:%s) error:%v", to, r, f, err)

			if err != nil {
				sent <- err
				return
			}
			if r.Type == ERROR_RESPONSE {
				errResp := r.Body.(ErrorResponse)
				err = errResp.DecodeResponseError()
				response = errResp.Payload
			} else {
				response = r.Body
			}
		}
		sent <- err
	}()
	select {
	case <-ctx.Done():
		err = ctx.Err()
		if err == context.DeadlineExceeded {
			err = SendTimeoutErr
		}
	case err = <-sent:
	}
	return
}

//Sign uses the agent' private key to sign the contents of doc
func (h *Holochain) Sign(doc []byte) (sig []byte, err error) {
	privKey := h.agent.PrivKey()
	sig, err = privKey.Sign(doc)
	if err != nil {
		return
	}
	return
}

//VerifySignature uses the signature, data(doc) and signatory's public key to Verify the sign in contents of doc
func (h *Holochain) VerifySignature(signature []byte, data string, pubKey ic.PubKey) (matches bool, err error) {

	matches, err = pubKey.Verify([]byte(data), signature)
	if err != nil {
		return
	}
	return
}

type QueryReturn struct {
	Hashes  bool
	Entries bool
	Headers bool
}

type QueryConstrain struct {
	EntryTypes []string
	Contains   string
	Equals     string
	Matches    string
	Count      int
	Page       int
}

type QueryOrder struct {
	Ascending bool
}

type QueryOptions struct {
	Return    QueryReturn
	Constrain QueryConstrain
	Order     QueryOrder
}

type QueryResult struct {
	Header *Header
	Entry  Entry
}

// Query scans the local chain and returns a collection of results based on the options specified
func (h *Holochain) Query(options *QueryOptions) (results []QueryResult, err error) {
	if options == nil {
		// default options
		options = &QueryOptions{}
		options.Return.Entries = true
	} else {
		// if no return options set, assume Entries
		if !options.Return.Entries && !options.Return.Hashes && !options.Return.Headers {
			options.Return.Entries = true
		}
	}
	var re *regexp.Regexp
	var equalsMap, containsMap map[string]interface{}
	var reMap map[string]*regexp.Regexp
	defs := make(map[string]*EntryDef)
	for i, header := range h.chain.Headers {

		var def *EntryDef
		var ok bool
		def, ok = defs[header.Type]
		if !ok {
			_, def, err = h.GetEntryDef(header.Type)
			if err != nil {
				return
			}
			defs[header.Type] = def
		}

		var skip bool
		if len(options.Constrain.EntryTypes) > 0 {
			skip = true
			for _, et := range options.Constrain.EntryTypes {
				if header.Type == et {
					skip = false
					break
				}
			}
		}
		if !skip && (options.Constrain.Equals != "" || options.Constrain.Contains != "" || options.Constrain.Matches != "") {
			var content string
			var contentMap map[string]interface{}
			if def.DataFormat == DataFormatJSON {
				contentMap = make(map[string]interface{})
				err = json.Unmarshal([]byte(h.chain.Entries[i].Content().(string)), &contentMap)
				if err != nil {
					return
				}
			} else {
				content = h.chain.Entries[i].Content().(string)
			}

			if !skip && options.Constrain.Equals != "" {
				if def.DataFormat == DataFormatJSON {
					if equalsMap == nil {
						equalsMap = make(map[string]interface{})
						err = json.Unmarshal([]byte(options.Constrain.Equals), &equalsMap)
						if err != nil {
							return
						}
					}
					skip = true
					for fieldName, fieldValue := range equalsMap {
						if contentMap[fieldName] == fieldValue {
							skip = false
							break
						}
					}
				} else {
					if content != options.Constrain.Equals {
						skip = true
					}
				}
			}
			if !skip && options.Constrain.Contains != "" {
				if def.DataFormat == DataFormatJSON {
					if containsMap == nil {
						containsMap = make(map[string]interface{})
						err = json.Unmarshal([]byte(options.Constrain.Contains), &containsMap)
						if err != nil {
							return
						}
					}
					skip = true
					for fieldName, fieldValue := range containsMap {
						if strings.Index(contentMap[fieldName].(string), fieldValue.(string)) >= 0 {
							skip = false
							break
						}
					}
				} else {
					if strings.Index(content, options.Constrain.Contains) < 0 {
						skip = true
					}
				}
			}
			if !skip && options.Constrain.Matches != "" {
				if def.DataFormat == DataFormatJSON {
					if reMap == nil {
						reMapStr := make(map[string]interface{})
						err = json.Unmarshal([]byte(options.Constrain.Matches), &reMapStr)
						if err != nil {
							return
						}
						reMap = make(map[string]*regexp.Regexp)
						for fieldName, fieldValue := range reMapStr {
							reMap[fieldName], err = regexp.Compile(fieldValue.(string))
							if err != nil {
								return
							}
						}
					}
					skip = true
					for fieldName, fieldRe := range reMap {
						if fieldRe.Match([]byte(contentMap[fieldName].(string))) {
							skip = false
							break
						}
					}

				} else {
					if re == nil {
						re, err = regexp.Compile(options.Constrain.Matches)
						if err != nil {
							return
						}
					}

					if !re.Match([]byte(content)) {
						skip = true
					}
				}

			}
		}

		if !skip {
			// we always need the header to be returned at this level.  The
			// Return values gets limited down to the actual info in the Ribosomes
			qr := QueryResult{Header: header}
			if options.Return.Entries {
				qr.Entry = h.chain.Entries[i]
			}
			if options.Order.Ascending {
				results = append([]QueryResult{qr}, results...)
			} else {
				results = append(results, qr)
			}
		}
	}
	if options.Constrain.Count > 0 {
		start := options.Constrain.Page * options.Constrain.Count
		if start >= len(results) {
			results = []QueryResult{}
		} else {
			end := start + options.Constrain.Count
			if end > len(results) {
				end = len(results)
			}
			results = results[start:end]
		}
	}
	return
}
