// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------

// Holochains are a distributed data store: DHT tightly bound to signed hash chains
// for provenance and data integrity.

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
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const Version int = 3
const VersionStr string = "3"

// Zome struct encapsulates logically related code, from "chromosome"
type Zome struct {
	Name        string
	Description string
	Code        string // file name of DNA code
	CodeHash    Hash
	Entries     map[string]EntryDef
	NucleusType string
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

// Config holds the non-DNA configuration for a holo-chain
type Config struct {
	Port            int
	PeerModeAuthor  bool
	PeerModeDHTNode bool
	BootstrapServer string
	Loggers         Loggers
}

// Holochain struct holds the full "DNA" of the holochain
type Holochain struct {
	Version          int
	Id               uuid.UUID
	Name             string
	Properties       map[string]string
	PropertiesSchema string
	HashType         string
	BasedOn          Hash // holochain hash for base schemas and code
	Zomes            map[string]*Zome
	RequiresVersion  int
	//---- private values not serialized; initialized on Load
	id             peer.ID // this is hash of the id, also used in the node
	dnaHash        Hash
	agentHash      Hash
	rootPath       string
	agent          Agent
	encodingFormat string
	hashSpec       HashSpec
	config         Config
	dht            *DHT
	node           *Node
	chain          *Chain // the chain itself
}

var debugLog Logger
var infoLog Logger

func Debug(m string) {
	debugLog.Log(m)
}

func Debugf(m string, args ...interface{}) {
	debugLog.Logf(m, args...)
}

func Info(m string) {
	infoLog.Log(m)
}

func Infof(m string, args ...interface{}) {
	infoLog.Logf(m, args...)
}

// Initialize function that must be called once at startup by any client app
func Initialize() {
	gob.Register(Header{})
	gob.Register(AgentEntry{})
	gob.Register(MetaEntry{})
	gob.Register(Hash{})
	gob.Register(PutReq{})
	gob.Register(GetReq{})
	gob.Register(MetaReq{})
	gob.Register(MetaQuery{})
	gob.Register(GossipReq{})
	gob.Register(Gossip{})
	gob.Register(ValidateResponse{})
	gob.Register(Put{})
	gob.Register(GobEntry{})
	gob.Register(MetaQueryResp{})
	gob.Register(TaggedHash{})

	RegisterBultinNucleii()

	infoLog.New(nil)
	debugLog.New(nil)

	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator

	DHTProtocol = Protocol{protocol.ID("/hc-dht/0.0.0"), DHTReceiver}
	ValidateProtocol = Protocol{protocol.ID("/hc-validate/0.0.0"), ValidateReceiver}
	GossipProtocol = Protocol{protocol.ID("/hc-gossip/0.0.0"), GossipReceiver}
}

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

// IsConfigured checks a directory for correctly set up holochain configuration files
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
		Id:              u,
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
	h.Zomes = make(map[string]*Zome)
	for i := range zomes {
		z := zomes[i]
		h.Zomes[z.Name] = &z
	}

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

	// try and get the agent from the holochain instance
	agent, err := LoadAgent(root)
	if err != nil {
		// get the default if not available
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

	h.chain, err = NewChainFromFile(h.hashSpec, h.DBPath()+"/"+StoreFileName+".dat")
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
		zpath := h.ZomePath(z)
		if !fileExists(zpath + "/" + z.Code) {
			return errors.New("DNA specified code file missing: " + z.Code)
		}
		for k := range z.Entries {
			e := z.Entries[k]
			sc := e.Schema
			if sc != "" {
				if !fileExists(zpath + "/" + sc) {
					return errors.New("DNA specified schema file missing: " + sc)
				}
				if strings.HasSuffix(sc, ".json") {
					if err = e.BuildJSONSchemaValidator(zpath); err != nil {
						return err
					}
					z.Entries[k] = e
				}
			}
		}
	}

	h.dht = NewDHT(h)

	return
}

// Activate fires up the holochain node
func (h *Holochain) Activate() (err error) {
	listenaddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", h.config.Port)
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
	for zomeName, z := range h.Zomes {
		var n Nucleus
		n, err = h.makeNucleus(z)
		if err == nil {
			err = n.ChainGenesis()
			if err != nil {
				err = fmt.Errorf("In '%s' zome: %s", zomeName, err.Error())
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
			h.Id = u

			// use the path as the name
			h.Name = filepath.Base(root)
		}

		// copy any UI files
		srcUiPath := srcPath + "/" + ChainUIDir
		if dirExists(srcUiPath) {
			if err = CopyDir(srcUiPath, h.UIPath()); err != nil {
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
			zpath := h.ZomePath(z)
			if err = os.MkdirAll(zpath, os.ModePerm); err != nil {
				return nil, err
			}
			if err = writeFile(zpath, z.Code, bs); err != nil {
				return
			}
			for k := range z.Entries {
				e := z.Entries[k]
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

// TestData holds a test entry for a chain
type TestData struct {
	Zome   string
	FnName string
	Input  interface{}
	Output string
	Err    string
	Regexp string
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
			{Name: "zySampleZome",
				Description: "this is a zygomas test zome",
				NucleusType: ZygoNucleusType,
				Entries: map[string]EntryDef{
					"evenNumbers": {Name: "evenNumbers", DataFormat: DataFormatRawZygo},
					"primes":      {Name: "primes", DataFormat: DataFormatJSON},
					"profile":     {Name: "profile", DataFormat: DataFormatJSON, Schema: "profile.json"},
				},
			},
			{Name: "jsSampleZome",
				Description: "this is a javascript test zome",
				NucleusType: JSNucleusType,
				Entries: map[string]EntryDef{
					"oddNumbers": {Name: "oddNumbers", DataFormat: DataFormatRawJS},
					"profile":    {Name: "profile", DataFormat: DataFormatJSON, Schema: "profile.json"},
				},
			},
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

		fixtures := [7]TestData{
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
				Err:    `Error calling 'commit': Invalid entry: {"Atype":"hash", "prime":4, "zKeyOrder":["prime"]}`},
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
(expose "getDNA" HC_STRING)
(defn getDNA [x] App_DNA_Hash)
(expose "exposedfn" HC_STRING)
(defn exposedfn [x] (concat "result: " x))
(expose "addEven" HC_STRING)
(defn addEven [x] (commit "evenNumbers" x))
(expose "addPrime" HC_JSON)
(defn addPrime [x] (commit "primes" x))
(defn validateCommit [entryType entry header sources]
  (validate entryType entry header sources))
(defn validatePut [entryType entry header sources]
  (validate entryType entry header sources))
(defn validate [entryType entry header sources]
  (cond (== entryType "evenNumbers")  (cond (== (mod entry 2) 0) true false)
        (== entryType "primes")  (isprime (hget entry %prime))
        (== entryType "profile") true
        false)
)
(defn validatePutMeta [baseType baseHash entryType entryHash tag sources] true)
(defn genesis [] true)
`
		code["jsSampleZome"] = `
expose("getProperty",HC.STRING);
function getProperty(x) {return property(x)};
expose("addOdd",HC.STRING);
function addOdd(x) {return commit("oddNumbers",x);}
expose("addProfile",HC.JSON);
function addProfile(x) {return commit("profile",x);}
function validatePut(entry_type,entry,header,sources) {
  return validate(entry_type,entry,header,sources);
}
function validateCommit(entry_type,entry,header,sources) {
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
function validatePutMeta(baseType,baseHash,ptrType,ptrHash,tag,sources){return true}
function genesis() {return true}
`

		testPath := root + "/test"
		if err = os.MkdirAll(testPath, os.ModePerm); err != nil {
			return nil, err
		}

		for n := range h.Zomes {
			z, _ := h.Zomes[n]

			zpath := h.ZomePath(z)

			if err = os.MkdirAll(zpath, os.ModePerm); err != nil {
				return nil, err
			}

			switch z.NucleusType {
			case JSNucleusType:
				z.Code = fmt.Sprintf("%s.js", z.Name)
			case ZygoNucleusType:
				z.Code = fmt.Sprintf("%s.zy", z.Name)
			default:
				err = fmt.Errorf("unknown nucleus type:%s", z.NucleusType)
				return
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

	h.chain, err = NewChainFromFile(h.hashSpec, h.DBPath()+"/"+StoreFileName+".dat")
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
		zpath := h.ZomePath(z)
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
	l, hash, header, err = h.chain.PrepareHeader(h.hashSpec, now, entryType, entry, h.agent.PrivKey())
	if err == nil {
		err = h.chain.addEntry(l, hash, header, entry)
	}

	if err == nil {
		Debugf("NewEntry of %s added as: %s (entry: %v)", entryType, header.EntryLink, entry)
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

// Validate scans back through a chain to the beginning confirming that the last header points to DNA
// This is actually kind of bogus on your own chain, because theoretically you put it there!  But
// if the holochain file was copied from somewhere you can consider this a self-check
func (h *Holochain) Validate(entriesToo bool) (valid bool, err error) {

	err = h.Walk(func(key *Hash, header *Header, entry Entry) (err error) {
		// confirm the correctness of the header hash

		var bH Hash
		bH, _, err = header.Sum(h.hashSpec)
		if err != nil {
			return
		}

		if !bH.Equal(key) {
			return errors.New("header hash doesn't match")
		}

		// @TODO check entry hashes Etoo if entriesToo set
		if entriesToo {

		}
		return nil
	}, entriesToo)
	if err == nil {
		valid = true
	}
	return
}

// GetEntryDef returns an EntryDef of the given name
// @TODO this makes the incorrect assumption that entry type strings are unique across zomes
func (h *Holochain) GetEntryDef(t string) (zome *Zome, d *EntryDef, err error) {
	for _, z := range h.Zomes {
		e, ok := z.Entries[t]
		if ok {
			zome = z
			d = &e
			break
		}
	}
	if d == nil {
		err = errors.New("no definition for entry type: " + t)
	}
	return
}

// ValidatePrepare does system level validation and structure creation before app validation
// It checks that entry is not nil, and that it conforms to the entry schema in the definition
// It returns the entry definition and a nucleus vm object on which to call the app validation
func (h *Holochain) ValidatePrepare(entryType string, entry Entry, sources []peer.ID) (d *EntryDef, srcs []string, n Nucleus, err error) {
	if entry == nil {
		err = errors.New("nil entry invalid")
		return
	}
	var z *Zome
	z, d, err = h.GetEntryDef(entryType)
	if err != nil {
		return
	}

	// see if there is a schema validator for the entry type and validate it if so
	if d.validator != nil {
		var input interface{}
		if d.DataFormat == DataFormatJSON {
			if err = json.Unmarshal([]byte(entry.Content().(string)), &input); err != nil {
				return
			}
		} else {
			input = entry
		}
		Debugf("Validating %v against schema", input)
		if err = d.validator.Validate(input); err != nil {
			return
		}
	}
	srcs = make([]string, 0)
	for _, s := range sources {
		srcs = append(srcs, peer.IDB58Encode(s))
	}
	// then run the nucleus (ie. "app" specific) validation rules
	n, err = h.makeNucleus(z)
	if err != nil {
		return
	}

	return
}

// ValidateCommit passes entry data to the chain's commit validation routine
// If the entry is valid err will be nil, otherwise it will contain some information about why the validation failed (or, possibly, some other system error)
func (h *Holochain) ValidateCommit(entryType string, entry Entry, header *Header, sources []peer.ID) (err error) {
	var d *EntryDef
	var srcs []string
	var n Nucleus
	d, srcs, n, err = h.ValidatePrepare(entryType, entry, sources)
	if err != nil {
		return
	}
	err = n.ValidateCommit(d, entry, header, srcs)
	return
}

// ValidatePut passes entry data to the chain's put validation routine
// If the entry is valid err will be nil, otherwise it will contain some information about why the validation failed (or, possibly, some other system error)
func (h *Holochain) ValidatePut(entryType string, entry Entry, header *Header, sources []peer.ID) (err error) {
	var d *EntryDef
	var srcs []string
	var n Nucleus
	d, srcs, n, err = h.ValidatePrepare(entryType, entry, sources)
	if err != nil {
		return
	}
	err = n.ValidatePut(d, entry, header, srcs)
	return
}

// ValidatePutMeta passes putmeta data to the chain's putmeta validation routine
// If the putmeta is valid err will be nil, otherwise it will contain some information about why the validation failed (or, possibly, some other system error)
func (h *Holochain) ValidatePutMeta(baseType string, baseHash Hash, ptrType string, ptrHash Hash, tag string, sources []peer.ID) (err error) {

	var z *Zome
	z, _, err = h.GetEntryDef(baseType)
	if err != nil {
		return
	}
	srcs := make([]string, 0)
	for _, s := range sources {
		srcs = append(srcs, peer.IDB58Encode(s))
	}
	// then run the nucleus (ie. "app" specific) validation rules
	var n Nucleus
	n, err = h.makeNucleus(z)
	if err != nil {
		return
	}
	err = n.ValidatePutMeta(baseType, baseHash.String(), ptrType, ptrHash.String(), tag, srcs)
	return
}

// Call executes an exposed function
func (h *Holochain) Call(zomeType string, function string, arguments interface{}) (result interface{}, err error) {
	n, err := h.MakeNucleus(zomeType)
	if err != nil {
		return
	}
	result, err = n.Call(function, arguments)
	return
}

// MakeNucleus creates a Nucleus object based on the zome type
func (h *Holochain) MakeNucleus(t string) (n Nucleus, err error) {
	z, ok := h.Zomes[t]
	if !ok {
		err = errors.New("unknown zome: " + t)
		return
	}
	n, err = h.makeNucleus(z)
	return
}

func (h *Holochain) makeNucleus(z *Zome) (n Nucleus, err error) {
	var code []byte
	zpath := h.ZomePath(z)
	code, err = readFile(zpath, z.Code)
	if err != nil {
		return
	}
	n, err = CreateNucleus(h, z.NucleusType, string(code))
	return
}

func LoadTestData(path string) (map[string][]TestData, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, errors.New("no test data found in: " + path + "/test")
	}

	re := regexp.MustCompile(`(.*)\.json`)
	var tests = make(map[string][]TestData)
	for _, f := range files {
		if f.Mode().IsRegular() {
			x := re.FindStringSubmatch(f.Name())
			if len(x) > 0 {
				name := x[1]

				var v []byte
				v, err = readFile(path, x[0])
				if err != nil {
					return nil, err
				}
				var t []TestData

				err = json.Unmarshal(v, &t)

				if err != nil {
					return nil, err
				}
				tests[name] = t
			}
		}
	}
	return tests, err
}

func ToString(input interface{}) string {
	// @TODO this should probably act according the function schema
	// not just the return value
	var output string
	switch t := input.(type) {
	case []byte:
		output = string(t)
	case string:
		output = t
	default:
		output = fmt.Sprintf("%v", t)
	}
	return output
}

func (h *Holochain) TestStringReplacements(input, r1, r2, r3 string) string {
	// get the top hash for substituting for %h% in the test expectation
	top := h.chain.Top().EntryLink

	var output string
	output = strings.Replace(input, "%h%", top.String(), -1)
	output = strings.Replace(output, "%r1%", r1, -1)
	output = strings.Replace(output, "%r2%", r2, -1)
	output = strings.Replace(output, "%r3%", r3, -1)
	output = strings.Replace(output, "%dna%", h.dnaHash.String(), -1)
	output = strings.Replace(output, "%agent%", h.agentHash.String(), -1)
	output = strings.Replace(output, "%agentstr%", string(h.Agent().Name()), -1)
	output = strings.Replace(output, "%key%", peer.IDB58Encode(h.id), -1)
	return output
}

// Test loops through each of the test files calling the functions specified
// This function is useful only in the context of developing a holochain and will return
// an error if the chain has already been started (i.e. has genesis entries)
func (h *Holochain) Test() []error {
	info := h.config.Loggers.TestInfo
	passed := h.config.Loggers.TestPassed
	failed := h.config.Loggers.TestFailed

	var err error
	var errs []error
	if h.Started() {
		err = errors.New("chain already started")
		return []error{err}
	}

	// load up the test files into the tests array
	var tests, errorLoad = LoadTestData(h.rootPath + "/" + ChainTestDir)
	if errorLoad != nil {
		return []error{errorLoad}
	}

	var lastResults [3]interface{}
	for name, ts := range tests {
		info.p("========================================")
		info.pf("Test: '%s' starting...", name)
		info.p("========================================")
		// setup the genesis entries
		err = h.Reset()
		_, err = h.GenChain()
		if err != nil {
			panic("gen err " + err.Error())
		}
		go h.dht.HandlePutReqs()
		for i, t := range ts {
			Debugf("------------------------------")
			info.pf("Test '%s' line %d: %s", name, i, t)
			time.Sleep(time.Millisecond * 10)
			if err == nil {
				testID := fmt.Sprintf("%s:%d", name, i)

				var input string
				switch inputType := t.Input.(type) {
				case string:
					input = t.Input.(string)
				case map[string]interface{}:
					inputByteArray, err := json.Marshal(t.Input)
					if err == nil {
						input = string(inputByteArray)
					}
				default:
					err = fmt.Errorf("Input was not an expected type: %T", inputType)
				}
				if err == nil {
					Debugf("Input before replacement: %s", input)
					r1 := strings.Trim(fmt.Sprintf("%v", lastResults[0]), "\"")
					r2 := strings.Trim(fmt.Sprintf("%v", lastResults[1]), "\"")
					r3 := strings.Trim(fmt.Sprintf("%v", lastResults[2]), "\"")
					input = h.TestStringReplacements(input, r1, r2, r3)
					Debugf("Input after replacement: %s", input)
					//====================
					var actualResult, actualError = h.Call(t.Zome, t.FnName, input)
					var expectedResult, expectedError = t.Output, t.Err
					var expectedResultRegexp = t.Regexp
					//====================
					lastResults[2] = lastResults[1]
					lastResults[1] = lastResults[0]
					lastResults[0] = actualResult
					if expectedError != "" {
						comparisonString := fmt.Sprintf("\nTest: %s\n\tExpected error:\t%v\n\tGot error:\t\t%v", testID, expectedError, actualError)
						if actualError == nil || (actualError.Error() != expectedError) {
							failed.pf("\n=====================\n%s\n\tfailed! m(\n=====================", comparisonString)
							err = fmt.Errorf(expectedError)
						} else {
							// all fine
							Debugf("%s\n\tpassed :D", comparisonString)
							err = nil
						}
					} else {
						if actualError != nil {
							errorString := fmt.Sprintf("\nTest: %s\n\tExpected:\t%s\n\tGot Error:\t\t%s\n", testID, expectedResult, actualError)
							err = fmt.Errorf(errorString)
							failed.pf(fmt.Sprintf("\n=====================\n%s\n\tfailed! m(\n=====================", errorString))
						} else {
							var resultString = ToString(actualResult)
							var match bool
							var comparisonString string
							if expectedResultRegexp != "" {
								Debugf("Test %s matching against regexp...", testID)
								expectedResultRegexp = h.TestStringReplacements(expectedResultRegexp, r1, r2, r3)
								comparisonString = fmt.Sprintf("\nTest: %s\n\tExpected regexp:\t%v\n\tGot:\t\t%v", testID, expectedResultRegexp, resultString)
								var matchError error
								match, matchError = regexp.MatchString(expectedResultRegexp, resultString)
								//match, matchError = regexp.MatchString("[0-9]", "7")
								if matchError != nil {
									Infof(err.Error())
								}
							} else {
								Debugf("Test %s matching against string...", testID)
								expectedResult = h.TestStringReplacements(expectedResult, r1, r2, r3)
								comparisonString = fmt.Sprintf("\nTest: %s\n\tExpected:\t%v\n\tGot:\t\t%v", testID, expectedResult, resultString)
								match = (resultString == expectedResult)
							}

							if match {
								Debugf("%s\n\tpassed! :D", comparisonString)
								passed.p("passed! ✔")
							} else {
								err = fmt.Errorf(comparisonString)
								failed.pf(fmt.Sprintf("\n=====================\n%s\n\tfailed! m(\n=====================", comparisonString))
							}
						}
					}
				}
			}

			if err != nil {
				errs = append(errs, err)
				err = nil
			}
		}
		// restore the state for the next test file
		e := h.Reset()
		if e != nil {
			panic(e)
		}
	}
	if len(errs) == 0 {
		passed.p(fmt.Sprintf("\n==================================================================\n\t\t+++++ All tests passed :D +++++\n=================================================================="))
	} else {
		failed.pf(fmt.Sprintf("\n==================================================================\n\t\t+++++ %d test(s) failed :( +++++\n==================================================================", len(errs)))
	}
	return errs
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

// Reset deletes all chain and dht data and resets data structures
func (h *Holochain) Reset() (err error) {

	h.dnaHash = Hash{}
	h.agentHash = Hash{}

	if h.chain.s != nil {
		h.chain.s.Close()
	}

	err = os.RemoveAll(h.DBPath() + "/" + ChainDataDir)
	if err != nil {
		panic(err)
	}

	err = os.RemoveAll(h.rootPath + "/" + DNAHashFileName)
	if err != nil {
		panic(err)
	}
	h.chain = NewChain()
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
		response, err = proto.Receiver(h, message)
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

// ---- These functions implement the required functions called by nuclei

// PutMeta services nucleus putmeta routines
// it both creates a meta entry and sends the putmeta request to the dht
func (h *Holochain) PutMeta(base string, metaHash string, tag string) (err error) {
	var baseHash Hash
	baseHash, err = NewHash(base)
	if err == nil {
		var metakey Hash
		metakey, err = NewHash(metaHash)
		if err == nil {
			me := MetaEntry{Base: baseHash, M: metakey, Tag: tag}
			e := GobEntry{C: me}
			_, mehd, err := h.NewEntry(time.Now(), MetaEntryType, &e)
			if err == nil {
				err = h.dht.SendPutMeta(MetaReq{Base: baseHash, M: metakey, T: mehd.EntryLink})
			}
		}
	}
	return
}

// GetMeta services nucleus getmata routines
func (h *Holochain) GetMeta(basestr string, tag string) (response interface{}, err error) {
	var base Hash
	base, err = NewHash(basestr)
	if err == nil {
		response, err = h.dht.SendGetMeta(MetaQuery{Base: base, T: tag})
	}
	return
}
