// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------

// Data integrity engine for distributed applications -- a validating monotonic
// DHT "backed" by authoritative hashchains for data provenance.
package holochain

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/google/uuid"
	peer "github.com/libp2p/go-libp2p-peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
	mh "github.com/multiformats/go-multihash"
	"io"
	"math/rand"
	"os"
	"time"
)

// Version is the numeric version number of the holochain library
const Version int = 12

// VersionStr is the textual version number of the holochain library
const VersionStr string = "12"

// Loggers holds the logging structures for the different parts of the system
type Loggers struct {
	App        Logger
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
	BootstrapServer string
	Loggers         Loggers
}

// Progenitor holds data on the creator of the DNA
type Progenitor struct {
	Identity string
	PubKey   []byte
}

// Holochain struct holds the full "DNA" of the holochain (all your app code for managing distributed data integrity)
type Holochain struct {
	//---- lowercase private values not serialized; initialized on Load
	nodeID         peer.ID // this is hash of the public key of the id and acts as the node address
	nodeIDStr      string  // this is just a cached version of the nodeID B58 string encoded
	dnaHash        Hash
	agentHash      Hash
	rootPath       string
	agent          Agent
	encodingFormat string
	hashSpec       HashSpec
	config         Config
	dht            *DHT
	nucleus        *Nucleus
	node           *Node
	chain          *Chain // This node's local source chain
}

func (h *Holochain) Nucleus() (n *Nucleus) {
	return h.nucleus
}

var debugLog Logger
var infoLog Logger

// Debug sends a string to the standard debug log
func Debug(m string) {
	debugLog.Log(m)
}

// Debugf sends a formatted string to the standard debug log
func Debugf(m string, args ...interface{}) {
	debugLog.Logf(m, args...)
}

// Info sends a string to the standard info log
func Info(m string) {
	infoLog.Log(m)
}

// Infof sends a formatted string to the standard info log
func Infof(m string, args ...interface{}) {
	infoLog.Logf(m, args...)
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

		RegisterBultinRibosomes()

		infoLog.New(nil)
		debugLog.New(nil)

		rand.Seed(time.Now().Unix()) // initialize global pseudo random generator

		ValidateProtocol = Protocol{protocol.ID("/hc-validate/0.0.0"), ValidateReceiver}
		GossipProtocol = Protocol{protocol.ID("/hc-gossip/0.0.0"), GossipReceiver}
		ActionProtocol = Protocol{protocol.ID("/hc-action/0.0.0"), ActionReceiver}
		_holochainInitialized = true
	}
}

// ZomePath returns the path to the zome dna data
// @todo sanitize the name value
func (h *Holochain) ZomePath(z *Zome) string {
	return h.DNAPath() + "/" + z.Name
}

// if the directories don't exist, make the place to store chains
func (h *Holochain) mkChainDirs() (err error) {
	if err = os.MkdirAll(h.DBPath(), os.ModePerm); err != nil {
		return err
	}
	if err = os.MkdirAll(h.DNAPath(), os.ModePerm); err != nil {
		return
	}
	if err = os.MkdirAll(h.UIPath(), os.ModePerm); err != nil {
		return
	}
	return
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

// Prepare sets up a holochain to run by:
// loading the schema validators, setting up a Network node and setting up the DHT
func (h *Holochain) Prepare() (err error) {

	err = h.nucleus.dna.check()
	if err != nil {
		return
	}

	if err = h.PrepareHashType(); err != nil {
		return
	}

	listenaddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", h.config.Port)
	h.node, err = NewNode(listenaddr, h.Agent().(*LibP2PAgent))
	if err != nil {
		return
	}

	h.dht = NewDHT(h)
	h.nucleus.h = h

	return
}

// Activate fires up the holochain node, starting node discovery and protocols
func (h *Holochain) Activate() (err error) {
	if h.config.EnableMDNS {
		err = h.node.EnableMDNSDiscovery(h, time.Second)
		if err != nil {
			return
		}
	}
	if h.config.BootstrapServer != "" {
		e := h.BSpost()
		if e != nil {
			h.dht.dlog.Logf("error in BSpost: %s", e.Error())
		}
		e = h.BSget()
		if e != nil {
			h.dht.dlog.Logf("error in BSget: %s", e.Error())
		}
	}
	if h.config.PeerModeDHTNode {
		if err = h.dht.Start(); err != nil {
			return
		}

	}
	if h.config.PeerModeAuthor {
		if err = h.nucleus.Start(); err != nil {
			return
		}
	}
	return
}

// UIPath returns a holochain UI path
func (h *Holochain) UIPath() string {
	return h.rootPath + "/" + ChainUIDir
}

// DBPath returns a holochain DB path
func (h *Holochain) DBPath() string {
	return h.rootPath + "/" + ChainDataDir
}

// DNAPath returns a holochain DNA path
func (h *Holochain) DNAPath() string {
	return h.rootPath + "/" + ChainDNADir
}

// TestPath returns the path to a holochain's test directory
func (h *Holochain) TestPath() string {
	return h.rootPath + "/" + ChainTestDir
}

// DNAHash returns the hash of the DNA entry which is also the holochain ID
func (h *Holochain) DNAHash() (id Hash) {
	return h.dnaHash.Clone()
}

// AgentHash returns the hash of the Agent entry
func (h *Holochain) AgentHash() (id Hash) {
	return h.agentHash.Clone()
}

// Top returns a hash of top header or err if not yet defined
func (h *Holochain) Top() (top Hash, err error) {
	tp := h.chain.Hashes[len(h.chain.Hashes)-1]
	top = tp.Clone()
	return
}

// Started returns true if the chain has been gened
func (h *Holochain) Started() bool {
	return h.DNAHash().String() != ""
}

// AddAgentEntry adds a new sys entry type setting the current agent data (identity and key)
func (h *Holochain) AddAgentEntry(revocation string) (headerHash, agentHash Hash, err error) {
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
// It assumes a properly set up .holochain sub-directory with a config file and
// keys for signing.  See GenDev()
func (h *Holochain) GenChain() (headerHash Hash, err error) {

	if h.Started() {
		err = mkErr("chain already started")
		return
	}

	defer func() {
		if err != nil {
			panic("cleanup after failed gen not implemented!  Error was: " + err.Error())
		}
	}()

	if err = h.Prepare(); err != nil {
		return
	}

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
	headerHash, agentHash, err = h.AddAgentEntry("") // revocation string is empty on initial Gen
	if err != nil {
		return
	}

	h.agentHash = agentHash

	if err = writeFile(h.rootPath, DNAHashFileName, []byte(h.dnaHash.String())); err != nil {
		return
	}

	err = h.dht.SetupDHT()
	if err != nil {
		return
	}

	h.nucleus.RunGenesis()

	return
}

func (h *Holochain) setupConfig() (err error) {
	if err = h.config.Loggers.App.New(nil); err != nil {
		return
	}
	if err = h.config.Loggers.DHT.New(nil); err != nil {
		return
	}
	if err = h.config.Loggers.Gossip.New(nil); err != nil {
		return
	}
	if err = h.config.Loggers.TestPassed.New(nil); err != nil {
		return
	}
	if err = h.config.Loggers.TestFailed.New(nil); err != nil {
		return
	}
	if err = h.config.Loggers.TestInfo.New(nil); err != nil {
		return
	}
	return
}

// EncodeDNA encodes a holochain's DNA to an io.Writer
func (h *Holochain) EncodeDNA(writer io.Writer) (err error) {
	return Encode(writer, h.encodingFormat, &h.nucleus.dna)
}

// NewEntry adds an entry and it's header to the chain and returns the header and it's hash
func (h *Holochain) NewEntry(now time.Time, entryType string, entry Entry) (hash Hash, header *Header, err error) {
	var l int
	l, hash, header, err = h.chain.PrepareHeader(now, entryType, entry, h.agent.PrivKey(), nil)
	if err == nil {
		err = h.chain.addEntry(l, hash, header, entry)
	}

	if err == nil {
		var e interface{} = entry
		if entryType == DNAEntryType {
			e = "<DNA>"
		}
		Debugf("NewEntry of %s added as: %s (entry: %v)", entryType, header.EntryLink, e)
	} else {
		Debugf("NewEntry of %s failed with: %s (entry: %v)", entryType, err, entry)
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
	for _, z := range h.nucleus.dna.Zomes {
		d, err = z.GetEntryDef(t)
		if err == nil {
			zome = &z
			return
		}
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

// Reset deletes all chain and dht data and resets data structures
func (h *Holochain) Reset() (err error) {

	h.dnaHash = Hash{}
	h.agentHash = Hash{}

	if h.chain.s != nil {
		h.chain.s.Close()
	}

	err = os.RemoveAll(h.DBPath())
	if err != nil {
		return
	}

	if err = os.MkdirAll(h.DBPath(), os.ModePerm); err != nil {
		return
	}
	h.chain, err = NewChainFromFile(h.hashSpec, h.DBPath()+"/"+StoreFileName)
	if err != nil {
		return
	}

	err = os.RemoveAll(h.rootPath + "/" + DNAHashFileName)
	if err != nil {
		panic(err)
	}
	if h.dht != nil {
		close(h.dht.puts)
		close(h.dht.gchan)
	}
	h.dht = NewDHT(h)

	return
}

// DHT exposes the DHT structure
func (h *Holochain) DHT() *DHT {
	return h.dht
}

// HashSpec exposes the hashSpec structure
func (h *Holochain) HashSpec() HashSpec {
	return h.hashSpec
}

// Send builds a message and either delivers it locally or over the network via node.Send
func (h *Holochain) Send(proto Protocol, to peer.ID, t MsgType, body interface{}) (response interface{}, err error) {
	message := h.node.NewMessage(t, body)
	if err != nil {
		return
	}
	f, err := message.Fingerprint()
	if err != nil {
		panic(fmt.Sprintf("error calculating fingerprint when sending message %v", message))
	}
	// if we are sending to ourselves we should bypass the network mechanics and call
	// the receiver directly
	if to == h.node.HashAddr {
		Debugf("Sending message (local):%v (fingerprint:%s)", message, f)
		response, err = proto.Receiver(h, message)
		Debugf("send result (local): %v (fp:%s)error:%v", response, f, err)
	} else {
		Debugf("Sending message (net):%v (fingerprint:%s)", message, f)
		var r Message
		r, err = h.node.Send(proto, to, message)
		Debugf("send result (net): %v (fp:%s) error:%v", r, f, err)

		if err != nil {
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
	return
}

func (h *Holochain) Chain() *Chain {
	return h.chain
}
