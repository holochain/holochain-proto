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
type AgentID string

type KeytypeType int

const (
	IPFS = iota
)

type Agent interface {
	ID() AgentID
	KeyType() KeytypeType
	GenKeys() error
	PrivKey() ic.PrivKey
	PubKey() ic.PubKey
}

type IPFSAgent struct {
	Id   AgentID
	Priv ic.PrivKey
}

func (a *IPFSAgent) ID() AgentID {
	return a.Id
}

func (a *IPFSAgent) KeyType() KeytypeType {
	return IPFS
}

func (a *IPFSAgent) PrivKey() ic.PrivKey {
	return a.Priv
}

func (a *IPFSAgent) PubKey() ic.PubKey {
	return a.Priv.GetPublic()
}

func (a *IPFSAgent) GenKeys() (err error) {
	var priv ic.PrivKey
	priv, _, err = ic.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return
	}
	a.Priv = priv
	return
}

func NewAgent(keyType KeytypeType, id AgentID) (agent Agent, err error) {
	switch keyType {
	case IPFS:
		a := IPFSAgent{
			Id: id,
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

// SaveAgent generates saves out the keys and agent id to the given directory
func SaveAgent(path string, agent Agent) (err error) {
	writeFile(path, AgentFileName, []byte(agent.ID()))
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
	id, err := readFile(path, AgentFileName)
	if err != nil {
		return
	}
	a := IPFSAgent{
		Id: AgentID(id),
	}
	k, err := readFile(path, PrivKeyFileName)
	if err != nil {
		return nil, err
	}
	a.Priv, err = ic.UnmarshalPrivateKey(k)
	if err != nil {
		return
	}
	agent = &a
	return
}
