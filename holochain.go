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
	"github.com/boltdb/bolt"
	"github.com/google/uuid"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	mh "github.com/multiformats/go-multihash"
	"github.com/op/go-logging"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const Version string = "0.0.1"

// KeyEntry structure for building KeyEntryType entries
type KeyEntry struct {
	ID      AgentID
	KeyType KeytypeType
	Key     []byte // marshaled public key
}

// Zome struct encapsulates logically related code, from "chromosome"
type Zome struct {
	Name        string
	Description string
	Code        string // file name of DNA code
	CodeHash    Hash
	Entries     map[string]EntryDef
	NucleusType string
}

// Config holds the non-DNA configuration for a holo-chain
type Config struct {
	Port            int
	PeerModeAuthor  bool
	PeerModeDHTNode bool
	BootstrapServer string
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
	//---- private values not serialized; initialized on Load
	path           string
	agent          Agent
	store          Persister
	encodingFormat string
	hashSpec       HashSpec
	config         Config
	dht            *DHT
	node           *Node
	chain          *Chain // the chain itself
}

var log *logging.Logger

// Register function that must be called once at startup by any client app
func Register(logger *logging.Logger) {
	gob.Register(Header{})
	gob.Register(KeyEntry{})
	gob.Register(Hash{})
	gob.Register(PutReq{})
	gob.Register(GetReq{})
	gob.Register(MetaReq{})
	gob.Register(MetaQuery{})
	RegisterBultinNucleii()
	RegisterBultinPersisters()
	log = logger
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
		err = errors.New("DNA not found")
		return
	}
	return
}

// IsConfigured checks a directory for correctly set up holochain configuration files
func (s *Service) IsConfigured(name string) (f string, err error) {
	path := s.Path + "/" + name

	f, err = findDNA(path)
	if err != nil {
		return
	}

	// found a format now check that there's a store
	p := path + "/" + StoreFileName + ".db"
	if !fileExists(p) {
		err = errors.New("chain store missing: " + p)
		return
	}

	return
}

// Load instantiates a Holochain instance
func (s *Service) Load(name string) (h *Holochain, err error) {
	f, err := s.IsConfigured(name)
	if err != nil {
		return
	}
	h, err = s.load(name, f)
	return
}

// New creates a new holochain structure with a randomly generated ID and default values
func New(agent Agent, path string, format string, zomes ...Zome) Holochain {
	u, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	h := Holochain{
		Id:             u,
		HashType:       "sha2-256",
		agent:          agent,
		path:           path,
		encodingFormat: format,
	}
	h.PrepareHashType()
	h.Zomes = make(map[string]*Zome)
	for i, _ := range zomes {
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

	path := s.Path + "/" + name
	var f *os.File
	f, err = os.Open(path + "/" + DNAFileName + "." + format)
	if err != nil {
		return
	}
	defer f.Close()
	h, err := DecodeDNA(f, format)
	if err != nil {
		return
	}
	h.path = path
	h.encodingFormat = format

	// load the config
	f, err = os.Open(path + "/" + ConfigFileName + "." + format)
	if err != nil {
		return
	}
	defer f.Close()
	err = Decode(f, format, &h.config)
	if err != nil {
		return
	}

	// try and get the agent from the holochain instance
	agent, err := LoadAgent(path)
	if err != nil {
		// get the default if not available
		agent, err = LoadAgent(filepath.Dir(path))
	}
	if err != nil {
		return
	}
	h.agent = agent

	h.store, err = CreatePersister(BoltPersisterName, path+"/"+StoreFileName+".db")
	if err != nil {
		return
	}

	err = h.store.Init()
	if err != nil {
		return
	}

	if err = h.PrepareHashType(); err != nil {
		return
	}

	h.chain, err = NewChainFromFile(h.hashSpec, path+"/"+StoreFileName+".dat")
	if err != nil {
		return
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
	if c, ok := mh.Names[h.HashType]; !ok {
		return fmt.Errorf("Unknown hash type: %s", h.HashType)
	} else {
		h.hashSpec.Code = c
		h.hashSpec.Length = -1
	}

	return
}

// Prepare sets up a holochain to run by:
// validating the DNA, loading the schema validators, setting up a Network node and setting up the DHT
func (h *Holochain) Prepare() (err error) {

	if err = h.PrepareHashType(); err != nil {
		return
	}
	for _, z := range h.Zomes {
		if !fileExists(h.path + "/" + z.Code) {
			return errors.New("DNA specified code file missing: " + z.Code)
		}
		for k := range z.Entries {
			e := z.Entries[k]
			sc := e.Schema
			if sc != "" {
				if !fileExists(h.path + "/" + sc) {
					return errors.New("DNA specified schema file missing: " + sc)
				} else {
					if strings.HasSuffix(sc, ".json") {
						if err = e.BuildJSONSchemaValidator(h.path); err != nil {
							return err
						}
						z.Entries[k] = e
					}
				}
			}
		}
	}

	listenaddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", h.config.Port)
	h.node, err = NewNode(listenaddr, h.Agent().PrivKey())
	if err != nil {
		return
	}

	h.dht = NewDHT(h)

	if h.config.PeerModeDHTNode {
		if err = h.dht.StartDHT(); err != nil {
			return
		}
	}
	if h.config.PeerModeAuthor {
		if err = h.node.StartSrc(h); err != nil {
			return
		}
	}
	return
}

// getMetaHash gets a value from the store that's a hash
func (h *Holochain) getMetaHash(key string) (hash Hash, err error) {
	v, err := h.store.GetMeta(key)
	if err != nil {
		return
	}
	hash.H = v
	if v == nil {
		err = mkErr("Meta key '" + key + "' uninitialized")
	}
	return
}

// Path returns a holochain path
func (h *Holochain) Path() string {
	return h.path
}

// ID returns a holochain ID hash or err if not yet defined
func (h *Holochain) ID() (id Hash, err error) {
	id, err = h.getMetaHash(IDMetaKey)
	return
}

// Top returns a hash of top header or err if not yet defined
func (h *Holochain) Top() (top Hash, err error) {
	tp, err := h.getMetaHash(TopMetaKey)
	top = tp.Clone()

	return
}

// Top returns a hash of top header of the given type or err if not yet defined
func (h *Holochain) TopType(t string) (top Hash, err error) {
	tp, err := h.getMetaHash(TopMetaKey + "_" + t)
	top = tp.Clone()
	return
}

// GenChain establishes a holochain instance by creating the initial genesis entries in the chain
// It assumes a properly set up .holochain sub-directory with a config file and
// keys for signing.  See GenDev()
func (h *Holochain) GenChain() (keyHash Hash, err error) {

	defer func() {
		if err != nil {
			panic("cleanup after failed gen not implemented!  Error was: " + err.Error())
		}
	}()

	_, err = h.ID()
	if err == nil {
		err = mkErr("chain already started")
		return
	}

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

	var k KeyEntry
	k.ID = h.agent.ID()
	k.KeyType = h.agent.KeyType()

	pk := h.agent.PrivKey().GetPublic()

	k.Key, err = ic.MarshalPublicKey(pk)
	if err != nil {
		return
	}

	e.C = k
	keyHash, _, err = h.NewEntry(time.Now(), KeyEntryType, &e)
	if err != nil {
		return
	}

	err = h.store.PutMeta(IDMetaKey, dnaHeader.EntryLink.H)
	if err != nil {
		return
	}

	// run the init functions of each zome
	for _, z := range h.Zomes {
		var n Nucleus
		n, err = h.makeNucleus(z)
		if err == nil {
			err = n.ChainGenesis()
			if err != nil {
				return
			}
		}
	}

	return
}

// Clone copies DNA files from a source
func (s *Service) Clone(srcPath string, path string) (hP *Holochain, err error) {
	hP, err = gen(path, func(path string) (hP *Holochain, err error) {

		format, err := findDNA(srcPath)
		if err != nil {
			return
		}

		f, err := os.Open(srcPath + "/" + DNAFileName + "." + format)
		if err != nil {
			return
		}
		defer f.Close()
		h, err := DecodeDNA(f, format)
		if err != nil {
			return
		}

		agent, err := LoadAgent(filepath.Dir(path))
		if err != nil {
			return
		}
		h.path = path
		h.agent = agent

		// make a config file
		err = makeConfig(h, s)
		if err != nil {
			return
		}

		// generate a new UUID
		u, err := uuid.NewUUID()
		if err != nil {
			return
		}
		h.Id = u

		if err = CopyDir(srcPath+"/ui", path+"/ui"); err != nil {
			return
		}

		if err = CopyFile(srcPath+"/schema_properties.json", path+"/schema_properties.json"); err != nil {
			return
		}

		if dirExists(srcPath + "/test") {
			if err = CopyDir(srcPath+"/test", path+"/test"); err != nil {
				return
			}
		}

		for _, z := range h.Zomes {
			var bs []byte
			bs, err = readFile(srcPath, z.Code)
			if err != nil {
				return
			}
			if err = writeFile(path, z.Code, bs); err != nil {
				return
			}
			for k := range z.Entries {
				e := z.Entries[k]
				sc := e.Schema
				if sc != "" {
					if err = CopyFile(srcPath+"/"+sc, path+"/"+sc); err != nil {
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
	Input  string
	Output string
	Err    string
}

func makeConfig(h *Holochain, s *Service) error {
	h.config.Port = DefaultPort
	h.config.PeerModeDHTNode = s.Settings.DefaultPeerModeDHTNode
	h.config.PeerModeAuthor = s.Settings.DefaultPeerModeAuthor

	p := h.path + "/" + ConfigFileName + "." + h.encodingFormat
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	return Encode(f, h.encodingFormat, &h.config)
}

// GenDev generates starter holochain DNA files from which to develop a chain
func (s *Service) GenDev(path string, format string) (hP *Holochain, err error) {
	hP, err = gen(path, func(path string) (hP *Holochain, err error) {
		agent, err := LoadAgent(filepath.Dir(path))
		if err != nil {
			return
		}

		zomes := []Zome{
			Zome{Name: "myZome",
				Description: "this is a zygomas test zome",
				NucleusType: ZygoNucleusType,
				Entries: map[string]EntryDef{
					"myData":  EntryDef{Name: "myData", DataFormat: "zygo"},
					"primes":  EntryDef{Name: "primes", DataFormat: "JSON"},
					"profile": EntryDef{Name: "profile", DataFormat: "JSON", Schema: "schema_profile.json"},
				},
			},
			Zome{Name: "jsZome",
				Description: "this is a javascript test zome",
				NucleusType: JSNucleusType,
				Entries: map[string]EntryDef{
					"myOdds":  EntryDef{Name: "myOdds", DataFormat: "js"},
					"profile": EntryDef{Name: "profile", DataFormat: "JSON", Schema: "schema_profile.json"},
				},
			},
		}

		h := New(agent, path, format, zomes...)

		err = makeConfig(&h, s)
		if err != nil {
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
		if err = writeFile(path, "schema_properties.json", []byte(schema)); err != nil {
			return
		}

		h.PropertiesSchema = "schema_properties.json"
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
		if err = writeFile(path, "schema_profile.json", []byte(schema)); err != nil {
			return
		}

		fixtures := [7]TestData{
			TestData{
				Zome:   "myZome",
				FnName: "addData",
				Input:  "2",
				Output: "%h%"},
			TestData{
				Zome:   "myZome",
				FnName: "addData",
				Input:  "4",
				Output: "%h%"},
			TestData{
				Zome:   "myZome",
				FnName: "addData",
				Input:  "5",
				Err:    "Error calling 'commit': Invalid entry: 5"},
			TestData{
				Zome:   "myZome",
				FnName: "addPrime",
				Input:  "{\"prime\":7}",
				Output: "\"%h%\""}, // quoted because return value is json
			TestData{
				Zome:   "myZome",
				FnName: "addPrime",
				Input:  "{\"prime\":4}",
				Err:    `Error calling 'commit': Invalid entry: {"Atype":"hash", "prime":4, "zKeyOrder":["prime"]}`},
			TestData{
				Zome:   "jsZome",
				FnName: "addProfile",
				Input:  `{"firstName":"Art","lastName":"Brock"}`,
				Output: `"%h%"`},
			TestData{
				Zome:   "jsZome",
				FnName: "getProperty",
				Input:  "_id",
				Output: "%id%"},
		}

		fixtures2 := [2]TestData{
			TestData{
				Zome:   "jsZome",
				FnName: "addOdd",
				Input:  "7",
				Output: "%h%"},
			TestData{
				Zome:   "jsZome",
				FnName: "addOdd",
				Input:  "2",
				Err:    "Invalid entry: 2"},
		}

		ui := `
<html>
  <head>
    <title>Test</title>
    <script type="text/javascript" src="http://code.jquery.com/jquery-latest.js"></script>
    <script type="text/javascript">
     function send() {
         $.post(
             "/fn/"+$('select[name=zome]').val()+"/"+$('select[name=fn]').val(),
             $('#data').val(),
             function(data) {
                 $("#result").html("result:"+data)
                 $("#err").html("")
             }
         ).error(function(response) {
             $("#err").html(response.responseText)
             $("#result").html("")
         })
         ;
     };
    </script>
  </head>
  <body>
    <select id="zome" name="zome">
      <option value="myZome">myZome</option>
    </select>
    <select id="fn" name="fn">
      <option value="addData">addData</option>
    </select>
    <input id="data" name="data">
    <button onclick="send();">Send</button>
    send an even number and get back a hash, send and odd end get a error

    <div id="result"></div>
    <div id="err"></div>
  </body>
</html>
`
		uiPath := path + "/ui"
		if err = os.MkdirAll(uiPath, os.ModePerm); err != nil {
			return nil, err
		}
		if err = writeFile(uiPath, "index.html", []byte(ui)); err != nil {
			return
		}

		code := make(map[string]string)
		code["myZome"] = `
(expose "exposedfn" STRING)
(defn exposedfn [x] (concat "result: " x))
(expose "addData" STRING)
(defn addData [x] (commit "myData" x))
(expose "addPrime" JSON)
(defn addPrime [x] (commit "primes" x))
(defn validate [entryType entry props]
  (cond (== entryType "myData")  (cond (== (mod entry 2) 0) true false)
        (== entryType "primes")  (isprime (hget entry %prime))
        (== entryType "profile") true
        false)
)
(defn genesis [] true)
`
		code["jsZome"] = `
expose("getProperty",HC.STRING);
function getProperty(x) {return property(x)};
expose("addOdd",HC.STRING);
function addOdd(x) {return commit("myOdds",x);}
expose("addProfile",HC.JSON);
function addProfile(x) {return commit("profile",x);}
function validate(entry_type,entry,props) {
if (entry_type=="myOdds") {
  return entry%2 != 0
}
if (entry_type=="profile") {
  return true
}
return false
}
function genesis() {return true}
`

		testPath := path + "/test"
		if err = os.MkdirAll(testPath, os.ModePerm); err != nil {
			return nil, err
		}

		for n, _ := range h.Zomes {
			z, _ := h.Zomes[n]
			switch z.NucleusType {
			case JSNucleusType:
				z.Code = fmt.Sprintf("zome_%s.js", z.Name)
			case ZygoNucleusType:
				z.Code = fmt.Sprintf("zome_%s.zy", z.Name)
			default:
				err = fmt.Errorf("unknown nucleus type:%s", z.NucleusType)
				return
			}

			c, _ := code[z.Name]
			if err = writeFile(path, z.Code, []byte(c)); err != nil {
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
func gen(path string, makeH func(path string) (hP *Holochain, err error)) (h *Holochain, err error) {
	if dirExists(path) {
		return nil, mkErr(path + " already exists")
	}
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, err
	}

	// cleanup the directory if we enounter an error while generating
	defer func() {
		if err != nil {
			os.RemoveAll(path)
		}
	}()

	h, err = makeH(path)
	if err != nil {
		return
	}

	h.Name = filepath.Base(path)

	h.chain, err = NewChainFromFile(h.hashSpec, path+"/"+StoreFileName+".dat")
	if err != nil {
		return
	}

	h.store, err = CreatePersister(BoltPersisterName, path+"/"+StoreFileName+".db")
	if err != nil {
		return
	}

	err = h.store.Init()
	if err != nil {
		return
	}

	err = h.SaveDNA(false)
	if err != nil {
		return
	}

	return
}

// EncodeDNA encodes a holochain's DNA to an io.Writer
func (h *Holochain) EncodeDNA(writer io.Writer) (err error) {
	return Encode(writer, h.encodingFormat, &h)
}

// SaveDNA writes the holochain DNA to a file
func (h *Holochain) SaveDNA(overwrite bool) (err error) {
	p := h.path + "/" + DNAFileName + "." + h.encodingFormat
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
		b, err = readFile(h.path, code)
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
				b, err = readFile(h.path, sc)
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
func (h *Holochain) NewEntry(now time.Time, t string, entry Entry) (hash Hash, header *Header, err error) {

	// this is extra for now.
	_, err = h.chain.AddEntry(h.hashSpec, now, t, entry, h.agent.PrivKey())
	if err != nil {
		return
	}

	// get the current top of the chain
	ph, err := h.Top()
	if err != nil {
		ph = NullHash()
	}

	// and also the the top entry of this type
	pth, err := h.TopType(t)
	if err != nil {
		pth = NullHash()
	}

	hash, header, err = newHeader(h.hashSpec, now, t, entry, h.agent.PrivKey(), ph, pth)
	if err != nil {
		return
	}

	// @TODO
	// we have to do this stuff because currently we are persisting immediatly.
	// instead we should be persisting from the Chain object.

	// encode the header for saving
	b, err := header.Marshal()
	if err != nil {
		return
	}
	// encode the entry into bytes
	m, err := entry.Marshal()
	if err != nil {
		return
	}

	err = h.store.Put(t, hash, b, header.EntryLink, m)

	return
}

// get low level access to entries/headers (only works inside a bolt transaction)
func get(hb *bolt.Bucket, eb *bolt.Bucket, key []byte, getEntry bool) (header Header, entry interface{}, err error) {
	v := hb.Get(key)

	err = header.Unmarshal(v, 34)
	if err != nil {
		return
	}
	if getEntry {
		v = eb.Get(header.EntryLink.H)
		var g GobEntry
		err = g.Unmarshal(v)
		if err != nil {
			return
		}
		entry = g.C
	}
	return
}

func (h *Holochain) Walk(fn func(key *Hash, h *Header, entry interface{}) error, entriesToo bool) (err error) {
	nullHash := NullHash()
	err = h.store.(*BoltPersister).DB().View(func(tx *bolt.Tx) error {
		hb := tx.Bucket([]byte(HeaderBucket))
		eb := tx.Bucket([]byte(EntryBucket))
		mb := tx.Bucket([]byte(MetaBucket))
		key := mb.Get([]byte(TopMetaKey))

		var keyH Hash
		var header Header
		var visited = make(map[string]bool)
		for err == nil && !bytes.Equal(nullHash.H, key) {
			keyH.H = key
			// build up map of all visited headers to prevent loops
			s := string(key)
			_, present := visited[s]
			if present {
				err = errors.New("loop detected in walk")
			} else {
				visited[s] = true
				var e interface{}
				header, e, err = get(hb, eb, key, entriesToo)
				if err == nil {
					err = fn(&keyH, &header, e)
					key = header.HeaderLink.H
				}
			}
		}
		if err != nil {
			return err
		}
		// if the last item doesn't gets us to bottom, i.e. the header who's entry link is
		// the same as ID key then, the chain is invalid...
		if !bytes.Equal(header.EntryLink.H, mb.Get([]byte(IDMetaKey))) {
			return errors.New("chain didn't end at DNA!")
		}
		return err
	})
	return
}

// Validate scans back through a chain to the beginning confirming that the last header points to DNA
// This is actually kind of bogus on your own chain, because theoretically you put it there!  But
// if the holochain file was copied from somewhere you can consider this a self-check
func (h *Holochain) Validate(entriesToo bool) (valid bool, err error) {

	err = h.Walk(func(key *Hash, header *Header, entry interface{}) (err error) {
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

// ValidateEntry passes an entry data to the chain's validation routine
// If the entry is valid err will be nil, otherwise it will contain some information about why the validation failed (or, possibly, some other system error)
func (h *Holochain) ValidateEntry(entryType string, entry Entry, props *ValidationProps) (err error) {

	if entry == nil {
		return errors.New("nil entry invalid")
	}

	z, d, err := h.GetEntryDef(entryType)
	if err != nil {
		return
	}

	// see if there is a schema validator for the entry type and validate it if so
	if d.validator != nil {
		var input interface{}
		if d.DataFormat == "JSON" {
			if err = json.Unmarshal([]byte(entry.Content().(string)), &input); err != nil {
				return
			}
		} else {
			input = entry
		}
		if err = d.validator.Validate(input); err != nil {
			return
		}
	}

	// then run the nucleus (ie. "app" specific) validation rules
	n, err := h.makeNucleus(z)
	if err != nil {
		return
	}
	err = n.ValidateEntry(d, entry, props)
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
	code, err = readFile(h.path, z.Code)
	if err != nil {
		return
	}
	n, err = CreateNucleus(h, z.NucleusType, string(code))
	return
}

// Test loops through each of the test files calling the functions specified
// This function is useful only in the context of developing a holochain and will return
// an error if the chain has already been started (i.e. has genesis entries)
func (h *Holochain) Test() error {
	_, err := h.ID()
	if err == nil {
		err = errors.New("chain already started")
		return err
	}
	p := h.path + "/test"
	files, err := ioutil.ReadDir(p)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return errors.New("no test data found in: " + h.path + "/test")
	}

	// load up the test files into the tests array
	re := regexp.MustCompile(`(.*)\.json`)
	var tests = make(map[string][]TestData)
	for _, f := range files {
		if f.Mode().IsRegular() {
			x := re.FindStringSubmatch(f.Name())
			if len(x) > 0 {
				name := x[1]

				var v []byte
				v, err = readFile(p, x[0])
				if err != nil {
					return err
				}
				var t []TestData
				err = json.Unmarshal(v, &t)
				if err != nil {
					return err
				}
				tests[name] = t
			}
		}
	}

	for name, ts := range tests {
		log.Debugf("Test: %s starting...", name)
		for i, t := range ts {
			// setup the genesis entries
			_, err = h.GenChain()
			if err == nil {
				testID := fmt.Sprintf("%s:%d", name, i)
				var result interface{}
				result, err = h.Call(t.Zome, t.FnName, t.Input)
				log.Debugf("Test: %s result:%v, Err:%v", testID, result, err)
				if t.Err != "" {
					log.Debugf("Test: %s expecting error %v", testID, t.Err)
					if err == nil {
						err = fmt.Errorf("Test: %s\n  Expected Error: %s\n  Got: nil\n", testID, t.Err)
					} else {
						if err.Error() != t.Err {
							err = fmt.Errorf("Test: %s\n  Expected Error: %s\n  Got Error: %s\n", testID, t.Err, err.Error())
						} else {
							err = nil
						}
					}
				} else {
					log.Debugf("Test: %s expecting output %v", testID, t.Output)
					if err != nil {
						err = fmt.Errorf("Test: %s\n  Expected: %s\n  Got Error: %s\n", testID, t.Output, err.Error())
					} else {

						// @TODO this should probably act according the function schema
						// not just the return value
						var r string
						switch t := result.(type) {
						case []byte:
							r = string(t)
						case string:
							r = t
						default:
							r = fmt.Sprintf("%v", t)
						}

						// get the top hash for substituting for %h% in the test expectation
						var top Hash
						top, _ = h.Top()
						o := strings.Replace(t.Output, "%h%", top.String(), -1)

						// get the id hash for substituting for %id% in the test expectation
						id, _ := h.ID()
						o = strings.Replace(o, "%id%", id.String(), -1)
						if r != o {
							err = fmt.Errorf("Test: %d\n  Expected: %v\n  Got: %v\n", i+1, o, r)
						}
					}
				}
			}
			// restore the state for the next test file
			e := h.store.Remove()
			if e != nil {
				panic(e)
			}
			e = h.store.Init()
			if e != nil {
				panic(e)
			}
			e = os.RemoveAll(h.path + "/" + StoreFileName + ".dat")
			if e != nil {
				panic(e)
			}

			if err != nil {
				return err
			}
		}
	}
	return err
}

// GetProperty returns the value of a DNA property
func (h *Holochain) GetProperty(prop string) (property string, err error) {
	if prop == ID_PROPERTY {
		var id Hash
		id, err = h.ID()
		if err != nil {
			property = ""
		} else {
			property = id.String()
		}
	} else if prop == AGENT_ID_PROPERTY {
		property = peer.IDB58Encode(h.node.HashAddr)
	} else if prop == AGENT_NAME_PROPERTY {
		property = string(h.Agent().ID())
	} else {
		property = h.Properties[prop]
	}
	return
}
