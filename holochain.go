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
	"github.com/BurntSushi/toml"
	"github.com/boltdb/bolt"
	_ "github.com/ghodss/yaml" // doesn't work!
	"github.com/google/uuid"
	ic "github.com/libp2p/go-libp2p-crypto"
	mh "github.com/multiformats/go-multihash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Version string = "0.0.1"

// Signing key structure for building KeyEntryType entries
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
}

// Holochain struct holds the full "DNA" of the holochain
type Holochain struct {
	Version   int
	Id        uuid.UUID
	Name      string
	GroupInfo map[string]string
	HashType  string
	BasedOn   Hash // holochain hash for base schemas and code
	Zomes     map[string]*Zome
	//---- private values not serialized; initialized on Load
	path           string
	agent          Agent
	store          Persister
	encodingFormat string
	hashCode       uint64
	hashLength     int
	config         Config
	dht            *DHT
	node           *Node
}

type Signature struct {
	S []byte
}

// Holochain entry header
type Header struct {
	Type        string
	Time        time.Time
	HeaderLink  Hash // link to previous header
	EntryLink   Hash // link to entry
	TypeLink    Hash // link to header of previous header of this type
	MySignature Signature
	Meta        interface{}
}

// Register function that must be called once at startup by any client app
func Register() {
	gob.Register(Header{})
	gob.Register(KeyEntry{})
	RegisterBultinNucleii()
	RegisterBultinPersisters()
}

//IsConfigured checks a directory for correctly set up holochain configuration files
func (s *Service) IsConfigured(name string) (h *Holochain, err error) {
	path := s.Path + "/" + name
	p := path + "/" + DNAFileName
	if !fileExists(p) {
		return nil, errors.New("missing " + p)
	}
	p = path + "/" + StoreFileName
	if !fileExists(p) {
		return nil, errors.New("chain store missing: " + p)
	}

	h, err = s.Load(name)

	return
}

// New creates a new holochain structure with a randomly generated ID and default values
func New(agent Agent, path string, zomes ...Zome) Holochain {
	u, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	h := Holochain{
		Id:             u,
		HashType:       "sha2-256",
		agent:          agent,
		path:           path,
		encodingFormat: "toml",
	}
	h.PrepareHashType()
	h.Zomes = make(map[string]*Zome)
	for _, z := range zomes {
		h.Zomes[z.Name] = &z
	}

	return h
}

// DecodeDNA decodes a Holochan structure from an io.Reader
func DecodeDNA(reader io.Reader, format string) (hP *Holochain, err error) {
	var h Holochain
	switch format {
	case "toml":
		_, err = toml.DecodeReader(reader, &h)
		/* unfortunately these don't work!
		case "json":
			dec := json.NewDecoder(reader)
			err = dec.Decode(&h)
		case "yaml":
			y, e := ioutil.ReadAll(reader)
			if e != nil {
				err = e
				return
			}
			err = yaml.Unmarshal(y, &h)
		*/
	default:
		err = errors.New("unknown DNA encoding format: " + format)
	}
	if err == nil {
		h.encodingFormat = format
		hP = &h
	}
	return
}

// Load unmarshals a holochain structure for the named chain in a service
func (s *Service) Load(name string) (hP *Holochain, err error) {

	path := s.Path + "/" + name

	f, err := os.Open(path + "/" + DNAFileName)
	if err != nil {
		return
	}
	defer f.Close()
	h, err := DecodeDNA(f, "toml")
	if err != nil {
		return
	}
	h.path = path

	// load the config
	_, err = toml.DecodeFile(path+"/"+ConfigFileName, &h.config)
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

	h.store, err = CreatePersister(BoltPersisterName, path+"/"+StoreFileName)
	if err != nil {
		return
	}

	err = h.store.Init()
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
		h.hashCode = c
		h.hashLength = -1
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
		for k, _ := range z.Entries {
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
	top, err = h.getMetaHash(TopMetaKey)
	return
}

// Top returns a hash of top header of the given type or err if not yet defined
func (h *Holochain) TopType(t string) (top Hash, err error) {
	top, err = h.getMetaHash(TopMetaKey + "_" + t)
	return
}

// GenChain establishes a holochain instance by creating the initial genesis entries in the chain
// It assumes a properly set up .holochain sub-directory with a config file and
// keys for signing.  See GenDev()
func (h *Holochain) GenChain() (keyHash Hash, err error) {

	_, err = h.ID()
	if err == nil {
		err = mkErr("chain already started")
		return
	}

	var buf bytes.Buffer
	err = h.EncodeDNA(&buf)

	e := GobEntry{C: buf.Bytes()}

	_, dnaHeader, err := h.NewEntry(time.Now(), DNAEntryType, &e)
	if err != nil {
		return
	}

	var k KeyEntry
	k.ID = h.agent.ID()
	k.KeyType = h.agent.KeyType()

	pk := h.agent.PrivKey().GetPublic()
	if err != nil {
		return
	}

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

	if err = h.Prepare(); err != nil {
		return
	}
	return
}

// GenFrom copies DNA files from a source
func (s *Service) GenFrom(src_path string, path string) (hP *Holochain, err error) {
	hP, err = gen(path, func(path string) (hP *Holochain, err error) {

		f, err := os.Open(src_path + "/" + DNAFileName)
		if err != nil {
			return
		}
		defer f.Close()
		h, err := DecodeDNA(f, "toml")
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

		if err = CopyDir(src_path+"/ui", path+"/ui"); err != nil {
			return
		}

		for _, z := range h.Zomes {
			var bs []byte
			bs, err = readFile(src_path, z.Code)
			if err != nil {
				return
			}
			if err = writeFile(path, z.Code, bs); err != nil {
				return
			}
			if err = CopyDir(src_path+"/test", path+"/test"); err != nil {
				return
			}
			for k, _ := range z.Entries {
				e := z.Entries[k]
				sc := e.Schema
				if sc != "" {
					if err = CopyFile(src_path+"/"+sc, path+"/"+sc); err != nil {
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
	return writeToml(h.path, ConfigFileName, h.config, false)
}

func (s *Service) GenDev(path string) (hP *Holochain, err error) {
	hP, err = gen(path, func(path string) (hP *Holochain, err error) {
		agent, err := LoadAgent(filepath.Dir(path))
		if err != nil {
			return
		}

		zomes := []Zome{
			Zome{Name: "myZome",
				Description: "zome desc",
				NucleusType: ZygoNucleusType,
				Entries: map[string]EntryDef{
					"myData":  EntryDef{Name: "myData", DataFormat: "zygo"},
					"primes":  EntryDef{Name: "primes", DataFormat: "JSON"},
					"profile": EntryDef{Name: "profile", DataFormat: "JSON", Schema: "schema_profile.json"},
				},
			},
		}

		h := New(agent, path, zomes...)

		err = makeConfig(&h, s)
		if err != nil {
			return
		}

		schema := `{
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

		fixtures := [5]TestData{
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
(defn validate [entry_type entry]
  (cond (== entry_type "myData")  (cond (== (mod entry 2) 0) true false)
        (== entry_type "primes")  (isprime (hget entry %prime))
        (== entry_type "profile") true
        false)
)
(defn validateChain [entry user_data] true)
`
		testPath := path + "/test"
		if err = os.MkdirAll(testPath, os.ModePerm); err != nil {
			return nil, err
		}

		for _, z := range h.Zomes {
			z.Code = fmt.Sprintf("zome_%s.zy", z.Name)
			c, _ := code[z.Name]
			if err = writeFile(path, z.Code, []byte(c)); err != nil {
				return
			}
			for i, d := range fixtures {
				fn := fmt.Sprintf("%d.zy", i)
				var j []byte
				j, err = json.Marshal(d)
				if err != nil {
					return
				}
				if err = writeFile(testPath, fn, j); err != nil {
					return
				}
			}
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
	h, err = makeH(path)
	if err != nil {
		return
	}

	h.Name = filepath.Base(path)

	h.store, err = CreatePersister(BoltPersisterName, path+"/"+StoreFileName)
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
// we use toml so that the DNA is human readable
func (h *Holochain) EncodeDNA(writer io.Writer) (err error) {
	switch h.encodingFormat {
	case "toml":
		enc := toml.NewEncoder(writer)
		err = enc.Encode(h)
		/* unfortunately these don't work!
		case "json":
			enc := json.NewEncoder(writer)
			err = enc.Encode(h)
		case "yaml":
			y, e := yaml.Marshal(h)
			if e != nil {
				err = e
				return
			}
			n, e := writer.Write(y)
			if e != nil {
				err = e
				return
			}
			if n != len(y) {
				err = errors.New("unable to write all bytes while encoding DNA")
			}
		*/
	default:
		err = errors.New("unknown DNA encoding format: " + h.encodingFormat)
	}
	return
}

// SaveDNA writes the holochain DNA to a file
func (h *Holochain) SaveDNA(overwrite bool) (err error) {
	p := h.path + "/" + DNAFileName
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
		err = z.CodeHash.Sum(h, b)
		if err != nil {
			return
		}
		for _, e := range z.Entries {
			sc := e.Schema
			if sc != "" {
				b, err = readFile(h.path, sc)
				if err != nil {
					return
				}
				err = e.SchemaHash.Sum(h, b)
				if err != nil {
					return
				}
			}
		}

	}
	err = h.SaveDNA(true)
	return
}

// NewEntry adds an entry and it's header to the chain and returns the header and it's hash
func (h *Holochain) NewEntry(now time.Time, t string, entry Entry) (hash Hash, header *Header, err error) {
	var hd Header
	hd.Type = t
	hd.Time = now

	// get the current top of the chain
	ph, err := h.Top()
	if err == nil {
		hd.HeaderLink = ph

	} else {
		hd.HeaderLink = NullHash()
	}

	// and also the the top entry of this type
	ph, err = h.TopType(t)
	if err == nil {
		hd.TypeLink = ph
	} else {
		hd.TypeLink = NullHash()
	}

	// now encode the entry into bytes
	m, err := entry.Marshal()
	if err != nil {
		return
	}

	// calculate the entry's hash and store it in the header
	err = hd.EntryLink.Sum(h, m)
	if err != nil {
		return
	}

	// sign the hash of the entry
	sig, err := h.agent.PrivKey().Sign(hd.EntryLink.H)
	if err != nil {
		return
	}
	hd.MySignature = Signature{S: sig}

	// encode the header and create a hash of it
	b, err := ByteEncoder(&hd)
	if err != nil {
		return
	}
	err = hash.Sum(h, b)
	if err != nil {
		return
	}

	err = h.store.Put(t, hash, b, hd.EntryLink, m)

	header = &hd
	return
}

// get low level access to entries/headers (only works inside a bolt transaction)
func get(hb *bolt.Bucket, eb *bolt.Bucket, key []byte, getEntry bool) (header Header, entry interface{}, err error) {
	v := hb.Get(key)

	err = ByteDecoder(v, &header)
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
		var b []byte
		b, err = ByteEncoder(&header)
		if err != nil {
			return err
		}

		var bH Hash
		err = bH.Sum(h, b)
		if err != nil {
			return
		}

		if bH.String() != (*key).String() {
			return errors.New("header hash doesn't match")
		}

		// TODO check entry hashes too if entriesToo set
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
func (h *Holochain) ValidateEntry(entry_type string, entry interface{}) (err error) {

	if entry == nil {
		return errors.New("nil entry invalid")
	}

	z, d, err := h.GetEntryDef(entry_type)
	if err != nil {
		return
	}

	// see if there is a schema validator for the entry type and validate it if so
	if d.validator != nil {
		var input interface{}
		if d.DataFormat == "JSON" {
			if err = json.Unmarshal([]byte(entry.(string)), &input); err != nil {
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
	err = n.ValidateEntry(d, entry)
	return
}

// Call executes an exposed function
func (h *Holochain) Call(zome_type string, function string, arguments interface{}) (result interface{}, err error) {
	n, err := h.MakeNucleus(zome_type)
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

	// setup the genesis entries
	h.GenChain()

	// and make sure the store gets reset to null after the test runs
	defer func() {
		err = h.store.Remove()
		if err != nil {
			panic(err)
		}
		err = h.store.Init()
		if err != nil {
			panic(err)
		}
	}()

	// load up the test files into the tests arrary
	re := regexp.MustCompile(`([0-9])+\.(.*)`)
	var tests = make(map[int]TestData)
	for _, f := range files {
		if f.Mode().IsRegular() {
			x := re.FindStringSubmatch(f.Name())
			if len(x) > 0 {
				var i int
				i, err = strconv.Atoi(x[1])
				if err != nil {
					return err
				}
				var v []byte
				v, err = readFile(p, x[0])
				if err != nil {
					return err
				}
				var t TestData
				err = json.Unmarshal(v, &t)
				if err != nil {
					return err
				}
				tests[i] = t
			}
		}
	}

	for i, t := range tests {
		result, err := h.Call(t.Zome, t.FnName, t.Input)

		if t.Err != "" {
			if err == nil {
				return errors.New(fmt.Sprintf("Test: %d\n  Expected Error: %s\n  Got: nil\n", i+1, t.Err))
			} else {

				if err.Error() != t.Err {
					return errors.New(fmt.Sprintf("Test: %d\n  Expected Error: %s\n  Got Error: %s\n", i+1, t.Err, err.Error()))
				}
				err = nil
			}
		} else {
			if err != nil {
				return errors.New(fmt.Sprintf("Test: %d\n  Expected: %s\n  Got Error: %s\n", i+1, t.Output, err.Error()))
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
				top, err := h.Top()
				if err != nil {
					return err
				}
				o := strings.Replace(t.Output, "%h%", top.String(), -1)
				if r != o {
					return errors.New(fmt.Sprintf("Test: %d\n  Expected: %v\n  Got: %v\n", i+1, o, r))
				}
			}
		}
	}
	return err
}
