// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file

// Holochains are a distributed data store: DHT tightly bound to signed hash chains
// for provenance and data integrity.
package holochain

import (
	_ "fmt"
	"os"
	"io/ioutil"
	"errors"
	"crypto/ecdsa"
	"crypto/elliptic"
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
	_ "github.com/jbenet/go-base58"
)

const Version string = "0.0.1"

// System settings, directory, and file names
const (
	DirectoryName string = ".holochain" // Directory for storing config data
	DNAFileName string = "dna.conf"     // Group context settings for holochain
	LocalFileName string = "local.conf" // Setting for your local data store
	SysFileName string = "system.conf"  // Server & System settings
	AgentFileName string = "agent.txt"  // User ID info
	PubKeyFileName string = "pub.key"   // ECDSA Signing key - public
	PrivKeyFileName string = "priv.key" // ECDSA Signing key - private
	ChainFileName string = "chain.db"   // Filename for local data store
)

// Active Subsystems: DHT, Datastore, network port
type Config struct {
	Port string
	PeerModeAuthor bool
	PeerModeDHTNode bool
}

// Unique user identifier in context of this holochain
type Agent string

// Holochain DNA settings
type Holochain struct {
	Id uuid.UUID
	ShortName string
	FullName string
	Description string
	HashType string
	Types []string
	Schemas map[string]string
	Validators map[string]string

	//---- private values not serialized
	path string
	agent Agent
	privKey *ecdsa.PrivateKey
}

// SHA256 hash of Entry's Content
type Hash [32]byte

// Holds content for a holochain
type Entry interface{}

// ECDSA signature of an Entry
type Signature struct {
	R big.Int
	S big.Int
}

// Holochain entry header
type Header struct {
	Time time.Time
	Type string
	HeaderLink Hash
	EntryLink Hash
	MySignature Signature
	Meta interface{}
}

//IsInitialized checks a path for a correctly set up .holochain directory
func IsInitialized(path string) bool {
	root := path+"/"+DirectoryName
	return dirExists(root) && fileExists(root+"/"+SysFileName) && fileExists(root+"/"+AgentFileName)
}

//IsConfigured checks a directory for correctly set up holochain configuration files
func IsConfigured(path string) error {
	p := path+"/"+DNAFileName
	if !fileExists(p) {return errors.New("missing "+p)}
	lh, err := Load(path)
	if err != nil {return err}

	//if !fileExists(path+"/"+ChainFileName) {return errors.New("data store missing")}

	SelfDescribingSchemas := map[string]bool {
		"JSON": true}
	for _,t := range lh.Types {
		s := lh.Schemas[t]
		if !SelfDescribingSchemas[s] {
			if !fileExists(path+"/"+s) {return errors.New("DNA specified schema missing: "+s)}
		}
		s = lh.Validators[t]
		if !fileExists(path+"/"+s) {return errors.New("DNA specified validator missing: "+s)}
	}
	return nil
}

// New creates a new holochain structure with a randomly generated ID and default values
func New(agent Agent ,key *ecdsa.PrivateKey,path string) Holochain {
	u,err := uuid.NewUUID()
	if err != nil {panic(err)}
	h := Holochain {
		Id:u,
		HashType: "SHA256",
		Types: []string{"myData"},
		Schemas: map[string]string{"myData":"JSON"},
		Validators: map[string]string{"myData":"valid_myData.js"},
		agent: agent,
		privKey: key,
		path: path,
	}
	return h
}

// Load creates a holochain structure from the configuration files
func Load(path string) (hP *Holochain,err error) {
	var h Holochain
	_,err = toml.DecodeFile(path+"/"+DNAFileName, &h)
	if err != nil {return}
	h.path = path
	agent,key,err := LoadSigner(filepath.Dir(path))
	if err != nil {return}
	h.agent = agent
	h.privKey = key
	hP = &h
	return
}

// MarshalPublicKey stores a PublicKey to a serialized x509 format file
func MarshalPublicKey(path string, file string, key *ecdsa.PublicKey) error {
	k,err := x509.MarshalPKIXPublicKey(key)
	if err != nil {return err}
	err = writeFile(path,file,k)
	return err
}

// UnmarshalPublicKey loads a PublicKey from the serialized x509 format file
func UnmarshalPublicKey(path string, file string) (key *ecdsa.PublicKey,err error) {
	k,err := readFile(path,file)
	if (err != nil) {return nil,err}
	kk,err := x509.ParsePKIXPublicKey(k)
	key = kk.(*ecdsa.PublicKey)
	return key,err
}

// MarshalPrivateKey stores a PublicKey to a serialized x509 format file
func MarshalPrivateKey(path string, file string, key *ecdsa.PrivateKey) error {
	k,err := x509.MarshalECPrivateKey(key)
	if err != nil {return err}
	err = writeFile(path,file,k)
	return err
}

// UnmarshalPrivateKey loads a PublicKey from the serialized x509 format file
func UnmarshalPrivateKey(path string, file string) (key *ecdsa.PrivateKey,err error) {
	k,err := readFile(path,file)
	if (err != nil) {return nil,err}
	key,err = x509.ParseECPrivateKey(k)
	return key,err
}

// GenKeys creates a new pub/priv key pair and stores them at the given path.
func GenKeys(path string) error {
	if fileExists(path+"/"+PrivKeyFileName) {return errors.New("keys already exist")}
	priv,err := ecdsa.GenerateKey(elliptic.P224(),rand.Reader)
	if err != nil {return err}

	err = MarshalPrivateKey(path,PrivKeyFileName,priv)
	if err != nil {return err}

	var pub *ecdsa.PublicKey
	pub = priv.Public().(*ecdsa.PublicKey)
	err = MarshalPublicKey(path,PubKeyFileName,pub)
	return err
}

// GenChain sets up a holochain by creating the initial genesis links.
// It assumes a properly set up .holochain sub-directory with a config file and
// keys for signing.  See Gen
func GenChain() (err error) {
	return errors.New("not implemented")
}

//Init initializes service defaults and a new key pair in the dirname directory
func Init(path string,agent Agent) error {
	p := path+"/"+DirectoryName
	if err := os.MkdirAll(p,os.ModePerm); err != nil {
		return err
	}
	c := Config {
		PeerModeAuthor:true,
	}
	err := writeToml(p,SysFileName,c)
	if err != nil {return err}

	writeFile(p,AgentFileName,[]byte(agent))
	if err != nil {return err}

	err = GenKeys(p)

	return err
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

	h.ShortName = filepath.Base(path)
	if err = writeToml(path,DNAFileName,h); err != nil {return}
	hP = &h
	//	if err = writeFile(path,"myData.cp",[]byte(s)); err != nil {return}  //if captain proto...
	s := `
function validateEntry(entry) {
    return true;
};
function validateChain(entry,user_data) {
    return true;
};
`
	if err = writeFile(path,"valid_myData.js",[]byte(s)); err != nil {return}
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
		return
	})
	if err !=nil {db.Close();db = nil}
	return
}

/*
func (e *Entry) Marshal() ([]byte,error) {
	return []byte("fish"),nil
}

type EntryMarshaler interface {
	 MarshalContent()
}

//MarshalContent serialized the content portion of Entry preparing to be hashed
func (e *Entry) MarshalContent() ([]byte,error) {
	return []byte("fish"),nil
}


// AddEntry stores the an entry by its Hash into the data store
func AdddEntry(db *bolt.DB,key EntryHash,entry *Entry) (err error) {
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Entries"))
		v,err := entry.Marshal()
		if err !=nil {return err}
		err = b.Put(key[:], v)
		return nil
	})
	return
}
*/

// NewEntry returns a hashed signed Entry
func (h *Holochain) NewEntry(now time.Time,t string,prevHeader Hash,entry interface{}) (hash Hash,header *Header,err error) {
	var hd Header
	hd.Type = t
	hd.HeaderLink = prevHeader
	hd.Time = now

	j,err := json.Marshal(entry)
	if err != nil {return}
	hd.EntryLink = Hash(sha256.Sum256([]byte(j)))

	r,s,err := ecdsa.Sign(rand.Reader,h.privKey,hd.EntryLink[:])
	if err != nil {return}
	hd.MySignature = Signature{R:*r,S:*s}

	j,err = json.Marshal(hd)
	hash = Hash(sha256.Sum256([]byte(j)))

	header = &hd
	return
}

// ConfiguredChains returns a list of the configured chains in the given holochain directory
func ConfiguredChains(root string) map[string]bool {
	files, _ := ioutil.ReadDir(root)
	chains := make(map[string]bool)
	for _, f := range files {
		if f.IsDir() && (IsConfigured(root+"/"+f.Name()) == nil) {
			chains[f.Name()] = true
		}
	}
	return chains
}

//----------------------------------------------------------------------------------------
// non exported utility functions

func writeToml(path string,file string,data interface{}) error {
	p := path+"/"+file
	if fileExists(p) {
		return mkErr(path+" already exists")
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	err = enc.Encode(data);
	return err
}

func writeFile(path string,file string,data []byte) error {
	p := path+"/"+file
	if fileExists(p) {
		return mkErr(path+" already exists")
	}
	f, err := os.Create(p)
	if err != nil {return err}

	defer f.Close()
	l,err := f.Write(data)
	if (err != nil) {return err}

	if (l != len(data)) {return mkErr("unable to write all data")}
	f.Sync()
	return err
}

func readFile(path string,file string) (data []byte, err error) {
	p := path+"/"+file
	data, err = ioutil.ReadFile(p)
	return data,err
}

func mkErr(err string) error {
	return errors.New("holochain: "+err)
}

func dirExists(name string) bool {
	info, err := os.Stat(name)
	return err == nil &&  info.Mode().IsDir();
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {return false}
	return info.Mode().IsRegular();
}
