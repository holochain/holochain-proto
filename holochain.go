// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------

// Data integrity engine for distributed applications -- a validating monotonic
// DHT "backed" by authoritative hashchains for data provenance.
package holochain

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
	mh "github.com/multiformats/go-multihash"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Version is the numeric version number of the holochain library
const Version int = 10

// VersionStr is the textual version number of the holochain library
const VersionStr string = "10"

// Zome struct encapsulates logically related code, from a "chromosome"
type Zome struct {
	Name        string
	Description string
	Code        string // file name of DNA code
	CodeHash    Hash
	Entries     []EntryDef
	NucleusType string
	Functions   []FunctionDef

	// cache for code
	code string
}

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
	PeerModeAuthor  bool
	PeerModeDHTNode bool
	BootstrapServer string
	Loggers         Loggers
}

// Holochain struct holds the full "DNA" of the holochain (all your app code for managing distributed data integrity)
type Holochain struct {
	Version          int
	ID               uuid.UUID
	Name             string
	Properties       map[string]string
	PropertiesSchema string
	HashType         string
	BasedOn          Hash // references hash of another holochain that these schemas and code are derived from
	Zomes            []Zome
	RequiresVersion  int
	//---- lowercase private values not serialized; initialized on Load
	id             peer.ID // this is hash of the id, also used in the node. @todo clarify id variable name?
	dnaHash        Hash
	agentHash      Hash
	rootPath       string
	agent          Agent
	encodingFormat string
	hashSpec       HashSpec
	config         Config
	dht            *DHT
	node           *Node
	chain          *Chain // This node's local source chain
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

// Initialize function that must be called once at startup by any peered app
func Initialize() {
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

	RegisterBultinNucleii()

	infoLog.New(nil)
	debugLog.New(nil)

	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator

	DHTProtocol = Protocol{protocol.ID("/hc-dht/0.0.0"), DHTReceiver}
	ValidateProtocol = Protocol{protocol.ID("/hc-validate/0.0.0"), ValidateReceiver}
	GossipProtocol = Protocol{protocol.ID("/hc-gossip/0.0.0"), GossipReceiver}
}

// Find the DNA files
func findDNA(path string) (f string, err error) {
	p := path + "/" + DNAFileName

	matches, err := filepath.Glob(p + ".*")
	if err != nil {
		return
	}
	for _, fn := range matches {
		s := strings.Split(fn, ".")
		f = s[len(s)-1]
		if f == "json" || f == "yaml" || f == "toml" {
			break
		}
		f = ""
	}

	if f == "" {
		err = fmt.Errorf("No DNA file in %s/", path)
		return
	}
	return
}

// ZomePath returns the path to the zome dna data
// @todo sanitize the name value
func (h *Holochain) ZomePath(z *Zome) string {
	return h.DNAPath() + "/" + z.Name
}

// IsConfigured checks a directory for correctly set up holochain configuration file
func (s *Service) IsConfigured(name string) (f string, err error) {
	root := s.Path + "/" + name

	f, err = findDNA(root + "/" + ChainDNADir)
	if err != nil {
		return
	}
	//@todo check other things?

	return
}

// Load instantiates a Holochain instance from disk
func (s *Service) Load(name string) (h *Holochain, err error) {
	f, err := s.IsConfigured(name)
	if err != nil {
		return
	}
	h, err = s.load(name, f)
	return
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
	h := Holochain{
		ID:              u,
		HashType:        "sha2-256",
		RequiresVersion: Version,
		agent:           agent,
		rootPath:        root,
		encodingFormat:  format,
	}

	// once the agent is set up we can calculate the id
	h.id, err = peer.IDFromPrivateKey(agent.PrivKey())
	if err != nil {
		panic(err)
	}

	h.PrepareHashType()
	h.Zomes = zomes

	return h
}

// DecodeDNA decodes a Holochain structure from an io.Reader
func DecodeDNA(reader io.Reader, format string) (hP *Holochain, err error) {
	var h Holochain
	err = Decode(reader, format, &h)
	if err != nil {
		return
	}
	hP = &h
	hP.encodingFormat = format

	return
}

// load unmarshals a holochain structure for the named chain and format
func (s *Service) load(name string, format string) (hP *Holochain, err error) {

	root := s.Path + "/" + name
	var f *os.File
	f, err = os.Open(root + "/" + ChainDNADir + "/" + DNAFileName + "." + format)
	if err != nil {
		return
	}
	defer f.Close()
	h, err := DecodeDNA(f, format)
	if err != nil {
		return
	}
	h.encodingFormat = format
	h.rootPath = root

	// load the config
	f, err = os.Open(root + "/" + ConfigFileName + "." + format)
	if err != nil {
		return
	}
	defer f.Close()
	err = Decode(f, format, &h.config)
	if err != nil {
		return
	}
	if err = h.setupConfig(); err != nil {
		return
	}

	// try and get the holochain-specific agent info
	agent, err := LoadAgent(root)
	if err != nil {
		// if not specified for this app, get the default from the Agent.txt file for all apps
		agent, err = LoadAgent(filepath.Dir(root))
	}
	if err != nil {
		return
	}
	h.agent = agent

	// once the agent is set up we can calculate the id
	h.id, err = peer.IDFromPrivateKey(agent.PrivKey())
	if err != nil {
		return
	}

	if err = h.PrepareHashType(); err != nil {
		return
	}

	h.chain, err = NewChainFromFile(h.hashSpec, h.DBPath()+"/"+StoreFileName)
	if err != nil {
		return
	}

	// if the chain has been started there should be a DNAHashFile which
	// we can load to check against the actual hash of the DNA entry
	var b []byte
	b, err = readFile(h.rootPath, DNAHashFileName)
	if err == nil {
		h.dnaHash, err = NewHash(string(b))
		if err != nil {
			return
		}
		// @TODO compare value from file to actual hash
	}

	if h.chain.Length() > 0 {
		h.agentHash = h.chain.Headers[1].EntryLink
	}
	if err = h.Prepare(); err != nil {
		return
	}

	hP = h
	return
}

// Agent exposes the agent element
func (h *Holochain) Agent() Agent {
	return h.agent
}

// PrepareHashType makes sure the given string is a correct multi-hash and stores
// the code and length to the Holochain struct
func (h *Holochain) PrepareHashType() (err error) {
	c, ok := mh.Names[h.HashType]
	if !ok {
		return fmt.Errorf("Unknown hash type: %s", h.HashType)
	}
	h.hashSpec.Code = c
	h.hashSpec.Length = -1
	return
}

// Prepare sets up a holochain to run by:
// validating the DNA, loading the schema validators, setting up a Network node and setting up the DHT
func (h *Holochain) Prepare() (err error) {

	if h.RequiresVersion > Version {
		err = fmt.Errorf("Chain requires Holochain version %d", h.RequiresVersion)
		return
	}

	if err = h.PrepareHashType(); err != nil {
		return
	}
	for _, z := range h.Zomes {
		zpath := h.ZomePath(&z)
		if !fileExists(zpath + "/" + z.Code) {
			fmt.Printf("%v", z)
			return errors.New("DNA specified code file missing: " + z.Code)
		}
		for i, e := range z.Entries {
			sc := e.Schema
			if sc != "" {
				if !fileExists(zpath + "/" + sc) {
					return errors.New("DNA specified schema file missing: " + sc)
				}
				if strings.HasSuffix(sc, ".json") {
					if err = e.BuildJSONSchemaValidator(zpath); err != nil {
						return err
					}
					z.Entries[i] = e
				}
			}
		}
	}

	h.dht = NewDHT(h)

	return
}

// Activate fires up the holochain node
func (h *Holochain) Activate() (err error) {
	listenaddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", h.config.Port)
	h.node, err = NewNode(listenaddr, h.id, h.Agent().PrivKey())
	if err != nil {
		return
	}

	if h.config.PeerModeDHTNode {
		if err = h.dht.StartDHT(); err != nil {
			return
		}
		e := h.BSpost()
		if e != nil {
			h.dht.dlog.Logf("error in BSpost: %s", e.Error())
		}
		e = h.BSget()
		if e != nil {
			h.dht.dlog.Logf("error in BSget: %s", e.Error())
		}
	}
	if h.config.PeerModeAuthor {
		if err = h.node.StartValidate(h); err != nil {
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

	var k AgentEntry
	k.Name = h.agent.Name()
	k.KeyType = h.agent.KeyType()

	pk := h.agent.PrivKey().GetPublic()

	k.Key, err = ic.MarshalPublicKey(pk)
	if err != nil {
		return
	}

	e.C = k
	var agentHeader *Header
	headerHash, agentHeader, err = h.NewEntry(time.Now(), AgentEntryType, &e)
	if err != nil {
		return
	}

	h.agentHash = agentHeader.EntryLink

	if err = writeFile(h.rootPath, DNAHashFileName, []byte(h.dnaHash.String())); err != nil {
		return
	}

	err = h.dht.SetupDHT()
	if err != nil {
		return
	}

	// run the init functions of each zome
	for _, z := range h.Zomes {
		var n Nucleus
		n, err = h.makeNucleus(&z)
		if err == nil {
			err = n.ChainGenesis()
			if err != nil {
				err = fmt.Errorf("In '%s' zome: %s", z.Name, err.Error())
				return
			}
		}
	}

	return
}

// Clone copies DNA files from a source directory
func (s *Service) Clone(srcPath string, root string, new bool) (hP *Holochain, err error) {
	hP, err = gen(root, func(root string) (hP *Holochain, err error) {

		srcDNAPath := srcPath + "/" + ChainDNADir
		format, err := findDNA(srcDNAPath)
		if err != nil {
			return
		}

		f, err := os.Open(srcDNAPath + "/" + DNAFileName + "." + format)
		if err != nil {
			return
		}
		defer f.Close()
		h, err := DecodeDNA(f, format)
		if err != nil {
			return
		}
		h.rootPath = root

		agent, err := LoadAgent(filepath.Dir(root))
		if err != nil {
			return
		}
		h.agent = agent

		// once the agent is set up we can calculate the id
		h.id, err = peer.IDFromPrivateKey(agent.PrivKey())
		if err != nil {
			return
		}

		// make a config file
		if err = makeConfig(h, s); err != nil {
			return
		}

		if new {
			// generate a new UUID
			var u uuid.UUID
			u, err = uuid.NewUUID()
			if err != nil {
				return
			}
			h.ID = u

			// use the path as the name
			h.Name = filepath.Base(root)
		}

		// copy any UI files
		srcUIPath := srcPath + "/" + ChainUIDir
		if dirExists(srcUIPath) {
			if err = CopyDir(srcUIPath, h.UIPath()); err != nil {
				return
			}
		}

		// copy any test files
		srcTestDir := srcPath + "/" + ChainTestDir
		if dirExists(srcTestDir) {
			if err = CopyDir(srcTestDir, root+"/"+ChainTestDir); err != nil {
				return
			}
		}

		// create the DNA directory and copy
		if err := os.MkdirAll(h.DNAPath(), os.ModePerm); err != nil {
			return nil, err
		}

		propertiesSchema := srcDNAPath + "/properties_schema.json"
		if fileExists(propertiesSchema) {
			if err = CopyFile(propertiesSchema, h.DNAPath()+"/properties_schema.json"); err != nil {
				return
			}
		}

		for _, z := range h.Zomes {
			var bs []byte
			srczpath := srcDNAPath + "/" + z.Name
			bs, err = readFile(srczpath, z.Code)
			if err != nil {
				return
			}
			zpath := h.ZomePath(&z)
			if err = os.MkdirAll(zpath, os.ModePerm); err != nil {
				return nil, err
			}
			if err = writeFile(zpath, z.Code, bs); err != nil {
				return
			}
			for _, e := range z.Entries {
				sc := e.Schema
				if sc != "" {
					if err = CopyFile(srczpath+"/"+sc, zpath+"/"+sc); err != nil {
						return
					}
				}
			}
		}

		hP = h
		return
	})
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

func makeConfig(h *Holochain, s *Service) (err error) {
	h.config = Config{
		Port:            DefaultPort,
		PeerModeDHTNode: s.Settings.DefaultPeerModeDHTNode,
		PeerModeAuthor:  s.Settings.DefaultPeerModeAuthor,
		BootstrapServer: s.Settings.DefaultBootstrapServer,
		Loggers: Loggers{
			App:        Logger{Format: "%{color:cyan}%{message}", Enabled: true},
			DHT:        Logger{Format: "%{color:yellow}%{time} DHT: %{message}"},
			Gossip:     Logger{Format: "%{color:blue}%{time} Gossip: %{message}"},
			TestPassed: Logger{Format: "%{color:green}%{message}", Enabled: true},
			TestFailed: Logger{Format: "%{color:red}%{message}", Enabled: true},
			TestInfo:   Logger{Format: "%{message}", Enabled: true},
		},
	}

	p := h.rootPath + "/" + ConfigFileName + "." + h.encodingFormat
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = Encode(f, h.encodingFormat, &h.config); err != nil {
		return
	}
	if err = h.setupConfig(); err != nil {
		return
	}
	return
}

// GenDev generates starter holochain DNA files from which to develop a chain
func (s *Service) GenDev(root string, format string) (hP *Holochain, err error) {
	hP, err = gen(root, func(root string) (hP *Holochain, err error) {
		agent, err := LoadAgent(filepath.Dir(root))
		if err != nil {
			return
		}

		zomes := []Zome{
			{
				Name:        "zySampleZome",
				Code:        "zySampleZome.zy",
				Description: "this is a zygomas test zome",
				NucleusType: ZygoNucleusType,
				Entries: []EntryDef{
					{Name: "evenNumbers", DataFormat: DataFormatRawZygo, Sharing: Public},
					{Name: "primes", DataFormat: DataFormatJSON, Sharing: Public},
					{Name: "profile", DataFormat: DataFormatJSON, Schema: "profile.json", Sharing: Public},
				},
				Functions: []FunctionDef{
					{Name: "getDNA", CallingType: STRING_CALLING},
					{Name: "addEven", CallingType: STRING_CALLING, Exposure: PUBLIC_EXPOSURE},
					{Name: "addPrime", CallingType: JSON_CALLING, Exposure: PUBLIC_EXPOSURE},
					{Name: "testStrFn1", CallingType: STRING_CALLING},
					{Name: "testStrFn2", CallingType: STRING_CALLING},
					{Name: "testJsonFn1", CallingType: JSON_CALLING},
					{Name: "testJsonFn2", CallingType: JSON_CALLING},
				},
			},
			{
				Name:        "jsSampleZome",
				Code:        "jsSampleZome.js",
				Description: "this is a javascript test zome",
				NucleusType: JSNucleusType,
				Entries: []EntryDef{
					{Name: "oddNumbers", DataFormat: DataFormatRawJS, Sharing: Public},
					{Name: "profile", DataFormat: DataFormatJSON, Schema: "profile.json", Sharing: Public},
					{Name: "rating", DataFormat: DataFormatLinks},
				},
				Functions: []FunctionDef{
					{Name: "getProperty", CallingType: STRING_CALLING},
					{Name: "addOdd", CallingType: STRING_CALLING, Exposure: PUBLIC_EXPOSURE},
					{Name: "addProfile", CallingType: JSON_CALLING, Exposure: PUBLIC_EXPOSURE},
					{Name: "testStrFn1", CallingType: STRING_CALLING},
					{Name: "testStrFn2", CallingType: STRING_CALLING},
					{Name: "testJsonFn1", CallingType: JSON_CALLING},
					{Name: "testJsonFn2", CallingType: JSON_CALLING},
				}},
		}

		h := NewHolochain(agent, root, format, zomes...)

		if err = h.mkChainDirs(); err != nil {
			return nil, err
		}

		// use the path as the name
		h.Name = filepath.Base(root)

		if err = makeConfig(&h, s); err != nil {
			return
		}

		schema := `{
	"title": "Properties Schema",
	"type": "object",
	"properties": {
		"description": {
			"type": "string"
		},
		"language": {
			"type": "string"
		}
	}
}`

		if err = writeFile(h.DNAPath(), "properties_schema.json", []byte(schema)); err != nil {
			return
		}

		h.PropertiesSchema = "properties_schema.json"
		h.Properties = map[string]string{
			"description": "a bogus test holochain",
			"language":    "en"}

		schema = `{
	"title": "Profile Schema",
	"type": "object",
	"properties": {
		"firstName": {
			"type": "string"
		},
		"lastName": {
			"type": "string"
		},
		"age": {
			"description": "Age in years",
			"type": "integer",
			"minimum": 0
		}
	},
	"required": ["firstName", "lastName"]
}`

		fixtures := [8]TestData{
			{
				Zome:   "zySampleZome",
				FnName: "addEven",
				Input:  "2",
				Output: "%h%"},
			{
				Zome:   "zySampleZome",
				FnName: "addEven",
				Input:  "4",
				Output: "%h%"},
			{
				Zome:   "zySampleZome",
				FnName: "addEven",
				Input:  "5",
				Err:    "Error calling 'commit': Invalid entry: 5"},
			{
				Zome:   "zySampleZome",
				FnName: "addPrime",
				Input:  "{\"prime\":7}",
				Output: "\"%h%\""}, // quoted because return value is json
			{
				Zome:   "zySampleZome",
				FnName: "addPrime",
				Input:  "{\"prime\":4}",
				Err:    `Error calling 'commit': Invalid entry: {"prime":4}`},
			{
				Zome:   "jsSampleZome",
				FnName: "addProfile",
				Input:  `{"firstName":"Art","lastName":"Brock"}`,
				Output: `"%h%"`},
			{
				Zome:   "zySampleZome",
				FnName: "getDNA",
				Input:  "",
				Output: "%dna%"},
			{
				Zome:     "zySampleZome",
				FnName:   "getDNA",
				Input:    "",
				Err:      "function not available",
				Exposure: PUBLIC_EXPOSURE,
			},
		}

		fixtures2 := [2]TestData{
			{
				Zome:   "jsSampleZome",
				FnName: "addOdd",
				Input:  "7",
				Output: "%h%"},
			{
				Zome:   "jsSampleZome",
				FnName: "addOdd",
				Input:  "2",
				Err:    "Invalid entry: 2"},
		}

		for fileName, fileText := range SampleUI {
			if err = writeFile(h.UIPath(), fileName, []byte(fileText)); err != nil {
				return
			}
		}

		code := make(map[string]string)
		code["zySampleZome"] = `
(defn testStrFn1 [x] (concat "result: " x))
(defn testStrFn2 [x] (+ (atoi x) 2))
(defn testJsonFn1 [x] (begin (hset x output: (* (-> x input:) 2)) x))
(defn testJsonFn2 [x] (unjson (raw "[{\"a\":\"b\"}]"))) (defn getDNA [x] App_DNA_Hash)
(defn addEven [x] (commit "evenNumbers" x))
(defn addPrime [x] (commit "primes" x))
(defn validateCommit [entryType entry header pkg sources]
  (validate entryType entry header sources))
(defn validatePut [entryType entry header pkg sources]
  (validate entryType entry header sources))
(defn validateMod [entryType hash newHash pkg sources] true)
(defn validateDel [entryType hash pkg sources] true)
(defn validate [entryType entry header sources]
  (cond (== entryType "evenNumbers")  (cond (== (mod entry 2) 0) true false)
        (== entryType "primes")  (isprime (hget entry %prime))
        (== entryType "profile") true
        false)
)
(defn validateLink [linkEntryType baseHash links pkg sources] true)
(defn validatePutPkg [entryType] nil)
(defn validateModPkg [entryType] nil)
(defn validateDelPkg [entryType] nil)
(defn validateLinkPkg [entryType] nil)
(defn genesis [] true)
`
		code["jsSampleZome"] = `
function testStrFn1(x) {return "result: "+x};
function testStrFn2(x){ return parseInt(x)+2};
function testJsonFn1(x){ x.output = x.input*2; return x;};
function testJsonFn2(x){ return [{a:'b'}] };

function getProperty(x) {return property(x)};
function addOdd(x) {return commit("oddNumbers",x);}
function addProfile(x) {return commit("profile",x);}
function validatePut(entry_type,entry,header,pkg,sources) {
  return validate(entry_type,entry,header,sources);
}
function validateMod(entry_type,hash,newHash,pkg,sources) {
  return true;
}
function validateDel(entry_type,hash,pkg,sources) {
  return true;
}
function validateCommit(entry_type,entry,header,pkg,sources) {
  if (entry_type == "rating") {return true}
  return validate(entry_type,entry,header,sources);
}
function validate(entry_type,entry,header,sources) {
  if (entry_type=="oddNumbers") {
    return entry%2 != 0
  }
  if (entry_type=="profile") {
    return true
  }
  return false
}
function validateLink(linkEntryType,baseHash,linkHash,tag,pkg,sources){return true}
function validatePutPkg(entry_type) {
  req = {};
  req[HC.PkgReq.Chain]=HC.PkgReq.ChainOpt.Full;
  return req;
}
function validateModPkg(entry_type) { return null}
function validateDelPkg(entry_type) { return null}
function validateLinkPkg(entry_type) { return null}

function genesis() {return true}
`

		testPath := root + "/test"
		if err = os.MkdirAll(testPath, os.ModePerm); err != nil {
			return nil, err
		}

		for _, z := range h.Zomes {

			zpath := h.ZomePath(&z)

			if err = os.MkdirAll(zpath, os.ModePerm); err != nil {
				return nil, err
			}

			c, _ := code[z.Name]
			if err = writeFile(zpath, z.Code, []byte(c)); err != nil {
				return
			}

			// both zomes have the same profile schma, this will be generalized for
			// scaffold building code.
			if err = writeFile(zpath, "profile.json", []byte(schema)); err != nil {
				return
			}

		}

		// write out the tests
		for i, d := range fixtures {
			fn := fmt.Sprintf("test_%d.json", i)
			var j []byte
			t := []TestData{d}
			j, err = json.Marshal(t)
			if err != nil {
				return
			}
			if err = writeFile(testPath, fn, j); err != nil {
				return
			}
		}

		// also write out some grouped tests
		fn := "grouped.json"
		var j []byte
		j, err = json.Marshal(fixtures2)
		if err != nil {
			return
		}
		if err = writeFile(testPath, fn, j); err != nil {
			return
		}
		hP = &h
		return
	})
	return
}

// gen calls a make function which should build the holochain structure and supporting files
func gen(root string, makeH func(root string) (hP *Holochain, err error)) (h *Holochain, err error) {
	if dirExists(root) {
		return nil, mkErr(root + " already exists")
	}
	if err := os.MkdirAll(root, os.ModePerm); err != nil {
		return nil, err
	}

	// cleanup the directory if we enounter an error while generating
	defer func() {
		if err != nil {
			os.RemoveAll(root)
		}
	}()

	h, err = makeH(root)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(h.DBPath(), os.ModePerm); err != nil {
		return nil, err
	}

	h.chain, err = NewChainFromFile(h.hashSpec, h.DBPath()+"/"+StoreFileName)
	if err != nil {
		return nil, err
	}

	err = h.SaveDNA(false)
	if err != nil {
		return nil, err
	}

	return
}

// EncodeDNA encodes a holochain's DNA to an io.Writer
func (h *Holochain) EncodeDNA(writer io.Writer) (err error) {
	return Encode(writer, h.encodingFormat, &h)
}

// SaveDNA writes the holochain DNA to a file
func (h *Holochain) SaveDNA(overwrite bool) (err error) {
	p := h.DNAPath() + "/" + DNAFileName + "." + h.encodingFormat
	if !overwrite && fileExists(p) {
		return mkErr(p + " already exists")
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	err = h.EncodeDNA(f)
	return
}

// GenDNAHashes generates hashes for all the definition files in the DNA.
// This function should only be called by developer tools at the end of the process
// of finalizing DNA development or versioning
func (h *Holochain) GenDNAHashes() (err error) {
	var b []byte
	for _, z := range h.Zomes {
		code := z.Code
		zpath := h.ZomePath(&z)
		b, err = readFile(zpath, code)
		if err != nil {
			return
		}
		err = z.CodeHash.Sum(h.hashSpec, b)
		if err != nil {
			return
		}
		for i, e := range z.Entries {
			sc := e.Schema
			if sc != "" {
				b, err = readFile(zpath, sc)
				if err != nil {
					return
				}
				err = e.SchemaHash.Sum(h.hashSpec, b)
				if err != nil {
					return
				}
				z.Entries[i] = e
			}
		}

	}
	err = h.SaveDNA(true)
	return
}

// NewEntry adds an entry and it's header to the chain and returns the header and it's hash
func (h *Holochain) NewEntry(now time.Time, entryType string, entry Entry) (hash Hash, header *Header, err error) {
	var l int
	l, hash, header, err = h.chain.PrepareHeader(h.hashSpec, now, entryType, entry, h.agent.PrivKey(), nil)
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
	for _, z := range h.Zomes {
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
	n, z, err := h.MakeNucleus(zomeType)
	if err != nil {
		return
	}
	fn, err := h.GetFunctionDef(z, function)
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

// MakeNucleus creates a Nucleus object based on the zome type
func (h *Holochain) MakeNucleus(t string) (n Nucleus, z *Zome, err error) {
	z, err = h.GetZome(t)
	if err != nil {
		return
	}
	n, err = h.makeNucleus(z)
	return
}

func (h *Holochain) makeNucleus(z *Zome) (n Nucleus, err error) {
	//check to see if we have a cached version of the code, otherwise read from disk
	if z.code == "" {
		zpath := h.ZomePath(z)
		var code []byte

		code, err = readFile(zpath, z.Code)
		if err != nil {
			return
		}
		z.code = string(code)
	}
	n, err = CreateNucleus(h, z.NucleusType, z.code)
	return
}

// GetProperty returns the value of a DNA property
func (h *Holochain) GetProperty(prop string) (property string, err error) {
	if prop == ID_PROPERTY || prop == AGENT_ID_PROPERTY || prop == AGENT_NAME_PROPERTY {
		ChangeAppProperty.Log()
	} else {
		property = h.Properties[prop]
	}
	return
}

// GetZome returns a zome structure given its name
func (h *Holochain) GetZome(zName string) (z *Zome, err error) {
	for _, zome := range h.Zomes {
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

// GetEntryDef returns the entry def structure
func (z *Zome) GetEntryDef(entryName string) (e *EntryDef, err error) {
	for _, def := range z.Entries {
		if def.Name == entryName {
			e = &def
			break
		}
	}
	if e == nil {
		err = errors.New("no definition for entry type: " + entryName)
	}
	return
}

// GetFunctionDef returns the exposed function spec for the given zome and function
func (h *Holochain) GetFunctionDef(zome *Zome, fnName string) (fn *FunctionDef, err error) {
	for _, f := range zome.Functions {
		if f.Name == fnName {
			fn = &f
			break
		}
	}
	if fn == nil {
		err = errors.New("unknown exposed function: " + fnName)
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
	// if we are sending to ourselves we should bypass the network mechanics and call
	// the receiver directly
	if to == h.node.HashAddr {
		Debugf("Sending message local:%v", message)
		response, err = proto.Receiver(h, message)
		Debugf("local send result: %v error:%v", response, err)
	} else {
		Debugf("Sending message net:%v", message)
		var r Message
		r, err = h.node.Send(proto, to, message)
		Debugf("send result: %v error:%v", r, err)

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
