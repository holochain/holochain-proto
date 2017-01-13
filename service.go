// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Service implements functions and data that provide Holochain services


package holochain

import (
	"os"
	"crypto/ecdsa"
	"github.com/BurntSushi/toml"
	"io/ioutil"
)

// System settings, directory, and file names
const (
	DirectoryName string = ".holochain" // Directory for storing config data
	DNAFileName string = "dna.conf"     // Group context settings for holochain
	LocalFileName string = "local.conf" // Setting for your local data store
	SysFileName string = "system.conf"  // Server & System settings
	AgentFileName string = "agent.txt"  // User ID info
	PubKeyFileName string = "pub.key"   // ECDSA Signing key - public
	PrivKeyFileName string = "priv.key" // ECDSA Signing key - private
	StoreFileName string = "chain.db"   // Filename for local data store

	DefaultPort = 6283

	DNAEntryType = "_dna"
	KeyEntryType = "_key"
)


// Holochain service configuration, i.e. Active Subsystems: DHT and/or Datastore, network port, etc
type Config struct {
	Port int
	PeerModeAuthor bool
	PeerModeDHTNode bool
}

// Holochain service data structure
type Service struct {
	Settings Config
	DefaultAgent Agent
	DefaultKey *ecdsa.PrivateKey
	Path string
}

//IsInitialized checks a path for a correctly set up .holochain directory
func IsInitialized(path string) bool {
	root := path+"/"+DirectoryName
	return dirExists(root) && fileExists(root+"/"+SysFileName) && fileExists(root+"/"+AgentFileName)
}

//Init initializes service defaults including a signing key pair for an agent
func Init(path string,agent Agent) (service *Service, err error) {
	p := path+"/"+DirectoryName
	err = os.MkdirAll(p,os.ModePerm)
	if err != nil {return}
	s := Service {
		Settings: Config{
			Port: DefaultPort,
			PeerModeAuthor:true,
		},
		DefaultAgent:agent,
		Path:p,
	}

	err = writeToml(p,SysFileName,s.Settings,false)
	if err != nil {return}

	writeFile(p,AgentFileName,[]byte(agent))
	if err != nil {return}

	k,err := GenKeys(p)
	if err !=nil {return}
	s.DefaultKey = k

	service = &s;
	return
}

func LoadService(path string) (service *Service,err error) {
	agent,key,err := LoadSigner(path)
	if err != nil {return}
	s := Service {
		Path:path,
		DefaultAgent:agent,
		DefaultKey:key,
	}

	_,err = toml.DecodeFile(path+"/"+SysFileName, &s.Settings)
	if err != nil {return}

	service = &s
	return
}

// ConfiguredChains returns a list of the configured chains for the given service
func (s *Service) ConfiguredChains() (chains map[string]*Holochain,err error) {
	files, err := ioutil.ReadDir(s.Path)
	if err != nil {return}
	chains = make(map[string]*Holochain)
	for _, f := range files {
		if f.IsDir() {
			h,err := s.IsConfigured(f.Name())
			if err == nil {
				chains[f.Name()] = h
			}
		}
	}
	return
}
