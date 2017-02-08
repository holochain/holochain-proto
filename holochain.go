// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------

// Holochains are a distributed data store: DHT tightly bound to signed hash chains
// for provenance and data integrity.
package holochain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/boltdb/bolt"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

const Version string = "0.0.1"

// Unique user identifier in context of this holochain
type Agent string

// Signing key structure for building KEYEntryType entries
type KeyEntry struct {
	ID  Agent
	Key []byte // marshaled x509 public key
}

// Holochain DNA settings
type Holochain struct {
	Version   int
	Id        uuid.UUID
	Name      string
	GroupInfo map[string]string
	HashType  string
	BasedOn   Hash // holochain hash for base schemas and code
	EntryDefs []EntryDef
	//---- private values not serialized; initialized on Load
	path    string
	agent   Agent
	privKey *ecdsa.PrivateKey
	store   Persister
}

// Holds an entry definition
type EntryDef struct {
	Name       string
	Schema     string // file name of schema or language schema directive
	Code       string // file name of DNA code
	SchemaHash Hash
	CodeHash   Hash
}

// Holds content for a holochain
type Entry interface {
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Content() interface{}
}

// GobEntry is a structure for implementing Gob encoding of Entry content
type GobEntry struct {
	C interface{}
}

// JSONEntry is a structure for implementing JSON encoding of Entry content
type JSONEntry struct {
	C interface{}
}

// ECDSA signature of an Entry
type Signature struct {
	R *big.Int
	S *big.Int
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

func SelfDescribingSchema(sc string) bool {
	SelfDescribingSchemas := map[string]bool{
		"JSON":         true,
		ZygoSchemaType: true,
	}
	return SelfDescribingSchemas[sc]
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
	if err != nil {
		return
	}

	for _, d := range h.EntryDefs {
		sc := d.Schema
		if !SelfDescribingSchema(sc) {
			if !fileExists(path + "/" + sc) {
				return nil, errors.New("DNA specified schema missing: " + sc)
			}
		}
		sc = d.Code
		if !fileExists(path + "/" + sc) {
			return nil, errors.New("DNA specified code missing: " + sc)
		}
	}
	return
}

// New creates a new holochain structure with a randomly generated ID and default values
func New(agent Agent, key *ecdsa.PrivateKey, path string, defs ...EntryDef) Holochain {
	u, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	h := Holochain{
		Id:        u,
		HashType:  "SHA256",
		EntryDefs: defs,
		agent:     agent,
		privKey:   key,
		path:      path,
	}

	return h
}

// Load unmarshals a holochain structure for the named chain in a service
func (s *Service) Load(name string) (hP *Holochain, err error) {
	var h Holochain

	path := s.Path + "/" + name

	_, err = toml.DecodeFile(path+"/"+DNAFileName, &h)
	if err != nil {
		return
	}
	h.path = path

	// try and get the agent/key from the holochain instance
	agent, key, err := LoadSigner(path)
	if err != nil {
		// get the default if not available
		agent, key, err = LoadSigner(filepath.Dir(path))
	}
	if err != nil {
		return
	}
	h.agent = agent
	h.privKey = key

	h.store, err = CreatePersister(BoltPersisterName, path+"/"+StoreFileName)
	if err != nil {
		return
	}

	err = h.store.Init()
	if err != nil {
		return
	}

	hP = &h
	return
}

// getMetaHash gets a value from the store that's a hash
func (h *Holochain) getMetaHash(key string) (hash Hash, err error) {
	v, err := h.store.GetMeta(key)
	if err != nil {
		return
	}
	copy(hash[:], v)
	if v == nil {
		err = mkErr("Meta key '" + key + "' uninitialized")
	}
	return
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
	k.ID = h.agent

	pk, err := x509.MarshalPKIXPublicKey(h.privKey.Public().(*ecdsa.PublicKey))
	if err != nil {
		return
	}
	k.Key = pk

	e.C = k
	keyHash, _, err = h.NewEntry(time.Now(), KeyEntryType, &e)
	if err != nil {
		return
	}

	err = h.store.PutMeta(IDMetaKey, dnaHeader.EntryLink[:])
	if err != nil {
		return
	}

	return
}

// Gen adds template files suitable for editing to the given path
func GenDev(path string) (hP *Holochain, err error) {
	var h Holochain
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, err
	}
	agent, key, err := LoadSigner(filepath.Dir(path))
	if err != nil {
		return
	}

	defs := []EntryDef{
		EntryDef{Name: "myData", Schema: "zygo"},
	}

	h = New(agent, key, path, defs...)

	h.Name = filepath.Base(path)
	//	if err = writeFile(path,"myData.cp",[]byte(s)); err != nil {return}  //if captain proto...

	entries := make(map[string][]string)
	mde := [2]string{"2", "4"}
	entries["myData"] = mde[:]

	code := make(map[string]string)
	code["myData"] = `
(defn validate [entry] (cond (== (mod entry 2) 0) true false))
(defn validateChain [entry user_data] true)
`
	testPath := path + "/test"
	if err := os.MkdirAll(testPath, os.ModePerm); err != nil {
		return nil, err
	}

	for idx, d := range defs {
		entry_type := d.Name
		fn := fmt.Sprintf("valid_%s.zy", entry_type)
		h.EntryDefs[idx].Code = fn
		v, _ := code[entry_type]
		if err = writeFile(path, fn, []byte(v)); err != nil {
			return
		}
		for i, e := range entries[entry_type] {
			fn = fmt.Sprintf("%d_%s.zy", i, entry_type)
			if err = writeFile(testPath, fn, []byte(e)); err != nil {
				return
			}
		}
	}

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

	hP = &h
	return
}

// EncodeDNA encodes a holochain's DNA to an io.Writer
// we use toml so that the DNA is human readable
func (h *Holochain) EncodeDNA(writer io.Writer) (err error) {
	enc := toml.NewEncoder(writer)
	err = enc.Encode(h)
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
	for _, d := range h.EntryDefs {
		sc := d.Schema
		if !SelfDescribingSchema(sc) {
			b, err = readFile(h.path, sc)
			if err != nil {
				return
			}
			d.SchemaHash = Hash(sha256.Sum256(b))
		}
		sc = d.Code
		b, err = readFile(h.path, sc)
		if err != nil {
			return
		}
		d.CodeHash = Hash(sha256.Sum256(b))
	}
	err = h.SaveDNA(true)
	return
}

//LoadSigner gets the agent and signing key from the specified directory
func LoadSigner(path string) (agent Agent, key *ecdsa.PrivateKey, err error) {
	a, err := readFile(path, AgentFileName)
	if err != nil {
		return
	}
	agent = Agent(a)
	key, err = UnmarshalPrivateKey(path, PrivKeyFileName)
	return
}

// ByteEncoder encodes anything using gob
func ByteEncoder(data interface{}) (b []byte, err error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(data)
	if err != nil {
		return
	}
	b = buf.Bytes()
	return
}

// ByteEncoder decodes data encoded by ByteEncoder
func ByteDecoder(b []byte, to interface{}) (err error) {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(to)
	return
}

// implementation of Entry interface with gobs
func (e *GobEntry) Marshal() (b []byte, err error) {
	b, err = ByteEncoder(&e.C)
	return
}
func (e *GobEntry) Unmarshal(b []byte) (err error) {
	err = ByteDecoder(b, &e.C)
	return
}
func (e *GobEntry) Content() interface{} { return e.C }

// implementation of Entry interface with JSON
func (e *JSONEntry) Marshal() (b []byte, err error) {
	j, err := json.Marshal(e.C)
	if err != nil {
		return
	}
	b = []byte(j)
	return
}
func (e *JSONEntry) Unmarshal(b []byte) (err error) {
	err = json.Unmarshal(b, &e.C)
	return
}
func (e *JSONEntry) Content() interface{} { return e.C }

// NewEntry adds an entry and it's header to the chain and returns the header and it's hash
func (h *Holochain) NewEntry(now time.Time, t string, entry Entry) (hash Hash, header *Header, err error) {
	var hd Header
	hd.Type = t
	hd.Time = now

	ph, err := h.Top()
	if err == nil {
		hd.HeaderLink = ph
	}
	ph, err = h.TopType(t)
	if err == nil {
		hd.TypeLink = ph
	}

	m, err := entry.Marshal()
	if err != nil {
		return
	}
	hd.EntryLink = Hash(sha256.Sum256(m))

	r, s, err := ecdsa.Sign(rand.Reader, h.privKey, hd.EntryLink[:])
	if err != nil {
		return
	}
	hd.MySignature = Signature{R: r, S: s}

	b, err := ByteEncoder(&hd)
	if err != nil {
		return
	}
	hash = Hash(sha256.Sum256(b))
	h.store.(*BoltPersister).DB().Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(EntryBucket))
		err = bkt.Put(hd.EntryLink[:], m)
		if err != nil {
			return err
		}

		bkt = tx.Bucket([]byte(HeaderBucket))
		v := hash[:]
		err = bkt.Put(v, b)
		if err != nil {
			return err
		}

		// don't use PutMeta because this has to be in the transaction
		bkt = tx.Bucket([]byte(MetaBucket))
		err = bkt.Put([]byte(TopMetaKey), v)
		if err != nil {
			return err
		}
		err = bkt.Put([]byte("top_"+t), v)
		if err != nil {
			return err
		}

		return nil
	})

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
		v = eb.Get(header.EntryLink[:])
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
	var nullHash Hash
	var nullHashBytes = nullHash[:]
	err = h.store.(*BoltPersister).DB().View(func(tx *bolt.Tx) error {
		hb := tx.Bucket([]byte(HeaderBucket))
		eb := tx.Bucket([]byte(EntryBucket))
		mb := tx.Bucket([]byte(MetaBucket))
		key := mb.Get([]byte(TopMetaKey))

		var keyH Hash
		var header Header
		var visited = make(map[string]bool)
		for err == nil && !bytes.Equal(nullHashBytes, key) {
			copy(keyH[:], key)
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
					key = header.HeaderLink[:]
				}
			}
		}
		if err != nil {
			return err
		}
		// if the last item doesn't gets us to bottom, i.e. the header who's entry link is
		// the same as ID key then, the chain is invalid...
		if !bytes.Equal(header.EntryLink[:], mb.Get([]byte(IDMetaKey))) {
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
		b, err := ByteEncoder(&header)
		if err != nil {
			return err
		}
		bh := sha256.Sum256(b)
		var bH Hash
		copy(bH[:], bh[:])
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
func (h *Holochain) GetEntryDef(t string) (d *EntryDef, err error) {
	for _, x := range h.EntryDefs {
		if x.Name == t {
			d = &x
			break
		}
	}
	if d == nil {
		err = errors.New("no definition for type: " + t)
	}
	return
}

// ValidateEntry passes an entry data to the chain's validation routine
// If the entry is valid err will be nil, otherwise it will contain some information about why the validation failed (or, possibly, some other system error)
func (h *Holochain) ValidateEntry(header *Header, entry interface{}) (err error) {

	if entry == nil {
		return errors.New("nil entry invalid")
	}
	v, err := h.MakeNucleus(header.Type)
	if err != nil {
		return
	}
	err = v.ValidateEntry(entry)
	return
}

// MakeNucleus creates a Nucleus object based on the entry type
func (h *Holochain) MakeNucleus(t string) (v Nucleus, err error) {
	d, err := h.GetEntryDef(t)
	if err != nil {
		return
	}
	var code []byte
	code, err = readFile(h.path, d.Code)
	if err != nil {
		return
	}

	// which nucleus to use is inferred from the schema type
	v, err = CreateNucleus(d.Schema, string(code))

	return
}

// Test validates test data against the current validation rules.
// This function is useful only in the context of developing a holochain and will return
// an error if the chain has already been started (i.e. has genesis entries)
func (h *Holochain) Test() (err error) {
	_, err = h.ID()
	if err == nil {
		err = errors.New("chain already started")
		return
	}
	p := h.path + "/test"
	files, err := ioutil.ReadDir(p)
	if err != err {
		return
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

	// load up the entries into hashes
	re := regexp.MustCompile(`([0-9])+_(.*)\.(.*)`)
	var entryTypes = make(map[int]string)
	var entryValues = make(map[int]string)
	for _, f := range files {
		if f.Mode().IsRegular() {
			x := re.FindStringSubmatch(f.Name())
			if len(x) > 0 {
				var i int
				i, err = strconv.Atoi(x[1])
				if err != nil {
					return
				}
				entryTypes[i] = x[2]
				var v []byte
				v, err = readFile(p, x[0])
				if err != nil {
					return
				}
				entryValues[i] = string(v)
			}
		}
	}

	var keys []int
	for k := range entryValues {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for k := range keys {
		idx := keys[k]
		e := GobEntry{C: entryValues[idx]}
		var header *Header
		_, header, err = h.NewEntry(time.Now(), entryTypes[idx], &e)
		if err != nil {
			return
		}
		//TODO: really we should be running h.Validate to test headers and genesis too
		err = h.ValidateEntry(header, e.C)
		if err != nil {
			return
		}
	}
	return
}
