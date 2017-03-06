// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// key generation and marshal/unmarshaling for holochains

package holochain

import (
	"crypto/rand"
	"errors"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
)

// Unique user identifier in context of this holochain
type AgentName string

type KeytypeType int

const (
	IPFS = iota
)

type Agent interface {
	Name() AgentName
	KeyType() KeytypeType
	GenKeys() error
	PrivKey() ic.PrivKey
	PubKey() ic.PubKey
}

type IPFSAgent struct {
	name AgentName
	priv ic.PrivKey
}

func (a *IPFSAgent) Name() AgentName {
	return a.name
}

func (a *IPFSAgent) KeyType() KeytypeType {
	return IPFS
}

func (a *IPFSAgent) PrivKey() ic.PrivKey {
	return a.priv
}

func (a *IPFSAgent) PubKey() ic.PubKey {
	return a.priv.GetPublic()
}

func (a *IPFSAgent) GenKeys() (err error) {
	var priv ic.PrivKey
	priv, _, err = ic.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return
	}
	a.priv = priv
	return
}

// NewAgent creates an agent structure of the given type
// Note: currently only IPFS agents are implemented
func NewAgent(keyType KeytypeType, name AgentName) (agent Agent, err error) {
	switch keyType {
	case IPFS:
		a := IPFSAgent{
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
	return
}

// LoadAgent gets the agent and signing key from the specified directory
func LoadAgent(path string) (agent Agent, err error) {
	name, err := readFile(path, AgentFileName)
	if err != nil {
		return
	}
	a := IPFSAgent{
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
