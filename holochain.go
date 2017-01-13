// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------

// Holochains are a distributed data store: DHT tightly bound to signed hash chains
// for provenance and data integrity.
package holochain

import (
	_ "fmt"
	"os"
	"bytes"
	"encoding/gob"
	"io"
	"errors"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/sha256"
	"math/big"
	"path/filepath"
	"time"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/BurntSushi/toml"
	"github.com/boltdb/bolt"
)

const Version string = "0.0.1"


// Unique user identifier in context of this holochain
type Agent string

// Signing key structure for building KEYEntryType entries
type KeyEntry struct {
	ID Agent
	Key []byte // marshaled x509 public key
}

// Holochain DNA settings
type Holochain struct {
	Version int
	Id uuid.UUID
	Name string
	GroupInfo map[string]string
	HashType string
	BasedOn Hash  // holochain hash for base schemas and validators
	Types []string
	Schemas map[string]string
	SchemaHashes map[string]Hash
	Validators map[string]string
	ValidatorHashes map[string]Hash
	//---- private values not serialized
	path string
	agent Agent
	privKey *ecdsa.PrivateKey
	db *bolt.DB
}

// Holds content for a holochain
type Entry interface {
	Marshal() ([]byte,error)
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
	Type string
	Time time.Time
	HeaderLink Hash
	EntryLink Hash
	MySignature Signature
	Meta interface{}
}

func SelfDescribingSchema(sc string) bool {
	SelfDescribingSchemas := map[string]bool {
		"JSON": true,
		"zygo": true,
	}
	return SelfDescribingSchemas[sc]

}
//IsConfigured checks a directory for correctly set up holochain configuration files
func (s *Service) IsConfigured(name string) (h *Holochain, err error) {
	path := s.Path+"/"+name
	p := path+"/"+DNAFileName
	if !fileExists(p) {return nil,errors.New("missing "+p)}
	p = path+"/"+StoreFileName
	if !fileExists(p) {return nil,errors.New("chain store missing: "+p)}

	h, err = s.Load(name)
	if err != nil {return}

	for _,t := range h.Types {
		sc := h.Schemas[t]
		if !SelfDescribingSchema(sc) {
			if !fileExists(path+"/"+sc) {return nil,errors.New("DNA specified schema missing: "+sc)}
		}
		sc = h.Validators[t]
		if !fileExists(path+"/"+sc) {return nil,errors.New("DNA specified validator missing: "+sc)}
	}
	return
}

// New creates a new holochain structure with a randomly generated ID and default values
func New(agent Agent ,key *ecdsa.PrivateKey,path string) Holochain {
	u,err := uuid.NewUUID()
	if err != nil {panic(err)}
	h := Holochain {
		Id:u,
		HashType: "SHA256",
		Types: []string{"myData"},
		Schemas: map[string]string{"myData":"zygo"},
		SchemaHashes:  map[string]Hash{},
		Validators: map[string]string{"myData":"valid_myData.zyg"},
		ValidatorHashes:  map[string]Hash{},
		agent: agent,
		privKey: key,
		path: path,
	}
	return h
}

// Load unmarshals a holochain structure for the named chain in a service
func (s *Service) Load(name string) (hP *Holochain,err error) {
	var h Holochain

	path := s.Path+"/"+name

	_,err = toml.DecodeFile(path+"/"+DNAFileName, &h)
	if err != nil {return}
	h.path = path

	// try and get the agent/key from the holochain instance
	agent,key,err := LoadSigner(path)
	if err != nil {
		// get the default if not available
		agent,key,err = LoadSigner(filepath.Dir(path))
	}
	if err != nil {return}
	h.agent = agent
	h.privKey = key

	h.db,err = OpenStore(path+"/"+StoreFileName)
	if err != nil {return}

	hP = &h
	return
}

const (IDMetaKey = "id")

// ID returns a holochain ID hash or empty has if not yet defined
func (h *Holochain) ID() (id Hash,err error) {
	err = h.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Meta"))
		v := b.Get([]byte(IDMetaKey))
		if v == nil {return mkErr("chain not started")}
		copy(id[:],v)
		return nil
	})
	return
}

// GenChain establishes a holochain instance by creating the initial genesis entries in the chain
// It assumes a properly set up .holochain sub-directory with a config file and
// keys for signing.  See GenDev()
func (h *Holochain) GenChain() (keyHash Hash,err error) {

	_,err = h.ID()
	if err == nil {
		err = mkErr("chain already started")
		return
	}

	var buf bytes.Buffer
	err = h.EncodeDNA(&buf)

	e := GobEntry{C:buf.Bytes()}
	var nullLink Hash
	dnaHeaderHash,dnaHeader,err := h.NewEntry(time.Now(),DNAEntryType,nullLink,&e)
	if err != nil {return}

	var k KeyEntry
	k.ID = h.agent

	pk,err := x509.MarshalPKIXPublicKey(h.privKey.Public().(*ecdsa.PublicKey))
	if err != nil {return}
	k.Key = pk

	e.C = k
	keyHash,_,err = h.NewEntry(time.Now(),KeyEntryType,dnaHeaderHash,&e)

	err = h.db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte("Meta"))
		err = b.Put([]byte(IDMetaKey), dnaHeader.EntryLink[:])
		return err
	})

	return
}

// Gen adds template files suitable for editing to the given path
func GenDev(path string) (hP *Holochain, err error) {
	var h Holochain
	if err := os.MkdirAll(path,os.ModePerm); err != nil {
		return nil,err;
	}
	agent,key,err := LoadSigner(filepath.Dir(path))
	if err != nil {return}
	h = New(agent,key,path)

	h.Name = filepath.Base(path)
	//	if err = writeFile(path,"myData.cp",[]byte(s)); err != nil {return}  //if captain proto...
	s := `
(defun validateEntry(entry) true)
(defun validateChain(entry user_data) true))
`
	if err = writeFile(path,"valid_myData.zyg",[]byte(s)); err != nil {return}

	h.db,err = OpenStore(path+"/"+StoreFileName)
	if err != nil {return}

	err = h.SaveDNA(false)
	if err != nil {return}

	hP = &h
	return
}

// EncodeDNA encodes a holochain's DNA to an io.Writer
// we use toml so that the DNA is human readable
func (h *Holochain) EncodeDNA(writer io.Writer) (err error) {
	enc := toml.NewEncoder(writer)
	err = enc.Encode(h);
	return
}

// SaveDNA writes the holochain DNA to a file
func (h *Holochain) SaveDNA(overwrite bool) (err error) {
	p := h.path+"/"+DNAFileName
	if !overwrite && fileExists(p) {
		return mkErr(p+" already exists")
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
	for _,t := range h.Types {
		sc := h.Schemas[t]
		if !SelfDescribingSchema(sc) {
			b,err = readFile(h.path,sc)
			if err != nil {return}
			h.SchemaHashes[t] = Hash(sha256.Sum256(b))
		}
		sc = h.Validators[t]
		b,err = readFile(h.path,sc)
		if err != nil {return}
		h.ValidatorHashes[t] = Hash(sha256.Sum256(b))
	}
	err = h.SaveDNA(true)
	return
}

//LoadSigner gets the agent and signing key from the specified directory
func LoadSigner(path string) (agent Agent ,key *ecdsa.PrivateKey,err error) {
	a,err := readFile(path,AgentFileName)
	if err != nil {return}
	agent = Agent(a)
	key,err = UnmarshalPrivateKey(path,PrivKeyFileName)
	return
}

//OpenStore sets up the datastore for use and returns a handle to it
func OpenStore(path string) (db *bolt.DB, err error) {
	db, err = bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {return}
	err = db.Update(func(tx *bolt.Tx) (err error) {
		_, err = tx.CreateBucketIfNotExists([]byte("Entries"))
		if err == nil {
			_, err = tx.CreateBucketIfNotExists([]byte("Headers"))
			if err == nil {
				_, err = tx.CreateBucketIfNotExists([]byte("Meta"))
			}
		}
		return
	})
	if err !=nil {db.Close();db = nil}
	return
}

// ByteEncoder encodes anything using gob
func ByteEncoder(data interface{}) (b []byte,err error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(data)
	if err != nil {return}
	b = buf.Bytes()
	return
}

// ByteEncoder decodes data encoded by ByteEncoder
func ByteDecoder(b []byte,to interface{}) (err error) {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(to)
	return
}

// implementation of Entry interface with gobs
func (e *GobEntry) Marshal() (b []byte,err error) {
	b,err = ByteEncoder(&e.C)
	return
}
func (e *GobEntry) Unmarshal(b []byte) (err error) {
	err = ByteDecoder(b,&e.C)
	return
}
func (e *GobEntry) Content() interface{} {return e.C}

// implementation of Entry interface with JSON
func (e *JSONEntry) Marshal() (b []byte,err error) {
	j,err := json.Marshal(e.C)
	if err != nil {return}
	b = []byte(j)
	return
}
func (e *JSONEntry) Unmarshal(b []byte) (err error) {
	err = json.Unmarshal(b, &e.C)
	return
}
func (e *JSONEntry) Content() interface{} {return e.C}

// NewEntry adds an entry and it's header to the chain and returns the header and it's hash
func (h *Holochain) NewEntry(now time.Time,t string,prevHeader Hash,entry Entry) (hash Hash,header *Header,err error) {
	var hd Header
	hd.Type = t
	hd.HeaderLink = prevHeader
	hd.Time = now

	m,err := entry.Marshal()
	if err != nil {return}
	hd.EntryLink = Hash(sha256.Sum256(m))

	r,s,err := ecdsa.Sign(rand.Reader,h.privKey,hd.EntryLink[:])
	if err != nil {return}
	hd.MySignature = Signature{R:r,S:s}

	b,err := ByteEncoder(&hd)
	if err !=nil {return}
	hash = Hash(sha256.Sum256(b))
	h.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("Entries"))
		err = bkt.Put(hd.EntryLink[:], m)
		if err != nil {return err}

		bkt = tx.Bucket([]byte("Headers"))
		err = bkt.Put(hash[:], b)
		if err != nil {return err}

		return nil
	})
	header = &hd
	return
}

// Get returns a header, and (optionally) it's entry if getEntry is true
func (h *Holochain) Get(hash Hash,getEntry bool) (header Header,entry interface{},err error){
	err = h.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Headers"))
		v := b.Get(hash[:])

		err := ByteDecoder(v,&header)
		if err != nil {return err}
		if getEntry {
			b = tx.Bucket([]byte("Entries"))
			v = b.Get(header.EntryLink[:])
			var g GobEntry
			err = g.Unmarshal(v)
			if err != nil {return err}
			entry = g.C
		}

		return nil
	})
	return
}

}
