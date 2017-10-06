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
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// AgentIdentity is the user's unique identity information in context of this holochain.
// it follows AgentIdentitySchema in DNA
type AgentIdentity string

type AgentType int

const (
	LibP2P = iota
)

// Agent abstracts the key behaviors and connection to a holochain node address
// Note that this is currently only a partial abstraction because the NodeID is always a libp2p peer.ID
// to complete the abstraction so we could use other libraries for p2p2 network transaction we
// would need to also abstract a matching NodeID type
type Agent interface {
	Identity() AgentIdentity
	SetIdentity(id AgentIdentity)
	AgentType() AgentType
	GenKeys(seed io.Reader) error
	PrivKey() ic.PrivKey
	PubKey() ic.PubKey
	NodeID() (peer.ID, string, error)
	AgentEntry(revocation Revocation) (AgentEntry, error)
}

type LibP2PAgent struct {
	identity AgentIdentity
	priv     ic.PrivKey
	pub      ic.PubKey // cached so as not to recalculate all the time
}

func (a *LibP2PAgent) Identity() AgentIdentity {
	return a.identity
}
func (a *LibP2PAgent) SetIdentity(id AgentIdentity) {
	a.identity = id
}

func (a *LibP2PAgent) AgentType() AgentType {
	return LibP2P
}

func (a *LibP2PAgent) PrivKey() ic.PrivKey {
	return a.priv
}

func (a *LibP2PAgent) PubKey() ic.PubKey {
	return a.pub
}

func (a *LibP2PAgent) GenKeys(seed io.Reader) (err error) {
	var priv ic.PrivKey
	if seed == nil {
		seed = rand.Reader
	}
	priv, _, err = ic.GenerateEd25519Key(seed)
	if err != nil {
		return
	}
	a.priv = priv
	a.pub = priv.GetPublic()
	return
}

func (a *LibP2PAgent) NodeID() (nodeID peer.ID, nodeIDStr string, err error) {
	nodeID, err = peer.IDFromPrivateKey(a.PrivKey())
	if err == nil {
		nodeIDStr = peer.IDB58Encode(nodeID)
	}
	return
}

func (a *LibP2PAgent) AgentEntry(revocation Revocation) (entry AgentEntry, err error) {

	entry = AgentEntry{
		Identity: a.Identity(),
	}
	if revocation != nil {
		entry.Revocation, err = revocation.Marshal()
		if err != nil {
			return
		}
	}
	entry.PublicKey, err = ic.MarshalPublicKey(a.PubKey())
	if err != nil {
		return
	}
	return
}

// NewAgent creates an agent structure of the given type
// Note: currently only IPFS agents are implemented
func NewAgent(agentType AgentType, identity AgentIdentity, seed io.Reader) (agent Agent, err error) {
	switch agentType {
	case LibP2P:
		a := LibP2PAgent{
			identity: identity,
		}
		err = a.GenKeys(seed)
		if err != nil {
			return
		}
		agent = &a
	default:
		err = fmt.Errorf("unknown key type: %d", agentType)
	}
	return
}

// SaveAgent saves out the keys and agent name to the given directory
func SaveAgent(path string, agent Agent) (err error) {
	WriteFile([]byte(agent.Identity()), path, AgentFileName)
	if err != nil {
		return
	}
	if FileExists(path, PrivKeyFileName) {
		return errors.New("keys already exist")
	}
	var k []byte
	k, err = agent.PrivKey().Bytes()
	if err != nil {
		return
	}
	err = WriteFile(k, path, PrivKeyFileName)
	os.Chmod(filepath.Join(path, PrivKeyFileName), OS_USER_R)
	return
}

// LoadAgent gets the agent identity and private key from the specified directory
// TODO confirm against chain?
func LoadAgent(path string) (agent Agent, err error) {
	var perms os.FileMode

	// TODO, make this check also work on windows instead of just bypassing!
	if runtime.GOOS != "windows" {
		perms, err = filePerms(path, PrivKeyFileName)
		if err != nil {
			return
		}
		if perms != OS_USER_R {
			err = errors.New(filepath.Join(path, PrivKeyFileName) + " file not read-only")
			return
		}
	}
	var identity []byte
	identity, err = ReadFile(path, AgentFileName)
	if err != nil {
		return
	}
	a := LibP2PAgent{
		identity: AgentIdentity(identity),
	}
	k, err := ReadFile(path, PrivKeyFileName)
	if err != nil {
		return nil, err
	}
	a.priv, err = ic.UnmarshalPrivateKey(k)
	if err != nil {
		return
	}
	a.pub = a.priv.GetPublic()
	agent = &a
	return
}
