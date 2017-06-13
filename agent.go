// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// key generation and marshal/unmarshaling and abstraction of agent for holochains

package holochain

import (
	"crypto/rand"
	"errors"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	"os"
)

// AgentName is the user's unique identifier in context of this holochain.
type AgentName string

type KeytypeType int

const (
	LibP2P = iota
)

// Agent abstracts the key behaviors and connection to a holochain node address
// Note that this is currently only a partial abstraction because the NodeID is always a libp2p peer.ID
// to complete the abstraction so we could use other libraries for p2p2 network transaction we
// would need to also abstract a matching NodeID type
type Agent interface {
	Name() AgentName
	KeyType() KeytypeType
	GenKeys() error
	PrivKey() ic.PrivKey
	PubKey() ic.PubKey
	NodeID() (peer.ID, string, error)
}

type LibP2PAgent struct {
	name AgentName
	priv ic.PrivKey
}

func (a *LibP2PAgent) Name() AgentName {
	return a.name
}

func (a *LibP2PAgent) KeyType() KeytypeType {
	return LibP2P
}

func (a *LibP2PAgent) PrivKey() ic.PrivKey {
	return a.priv
}

func (a *LibP2PAgent) PubKey() ic.PubKey {
	return a.priv.GetPublic()
}

func (a *LibP2PAgent) GenKeys() (err error) {
	var priv ic.PrivKey
	priv, _, err = ic.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return
	}
	a.priv = priv
	return
}

func (a *LibP2PAgent) NodeID() (nodeID peer.ID, nodeIDStr string, err error) {
	nodeID, err = peer.IDFromPrivateKey(a.PrivKey())
	if err == nil {
		nodeIDStr = peer.IDB58Encode(nodeID)
	}
	return
}

// NewAgent creates an agent structure of the given type
// Note: currently only IPFS agents are implemented
func NewAgent(keyType KeytypeType, name AgentName) (agent Agent, err error) {
	switch keyType {
	case LibP2P:
		a := LibP2PAgent{
			name: name,
		}
		err = a.GenKeys()
		if err != nil {
			return
		}
		agent = &a
	default:
		err = fmt.Errorf("unknown key type: %d", keyType)
	}
	return
}

// SaveAgent saves out the keys and agent name to the given directory
func SaveAgent(path string, agent Agent) (err error) {
	writeFile(path, AgentFileName, []byte(agent.Name()))
	if err != nil {
		return
	}
	if fileExists(path + "/" + PrivKeyFileName) {
		return errors.New("keys already exist")
	}
	var k []byte
	k, err = agent.PrivKey().Bytes()
	if err != nil {
		return
	}
	err = writeFile(path, PrivKeyFileName, k)
	os.Chmod(path+"/"+PrivKeyFileName, OS_USER_R)
	return
}

// LoadAgent gets the agent and signing key from the specified directory
func LoadAgent(path string) (agent Agent, err error) {
	var perms os.FileMode
	perms, err = filePerms(path, PrivKeyFileName)
	if perms != OS_USER_R {
		err = errors.New(path + "/" + PrivKeyFileName + " file not read-only")
		return
	}
	var name []byte
	name, err = readFile(path, AgentFileName)
	if err != nil {
		return
	}
	a := LibP2PAgent{
		name: AgentName(name),
	}
	k, err := readFile(path, PrivKeyFileName)
	if err != nil {
		return nil, err
	}
	a.priv, err = ic.UnmarshalPrivateKey(k)
	if err != nil {
		return
	}
	agent = &a
	return
}
