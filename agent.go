// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// key generation and marshal/unmarshaling for holochains

package holochain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	//	ic "github.com/libp2p/go-libp2p-crypto"
)

// Unique user identifier in context of this holochain
type AgentID string

// NewAgent generates keys and saves them to the given directory
func NewAgent(path string, agent AgentID) (k *ecdsa.PrivateKey, err error) {
	writeFile(path, AgentFileName, []byte(agent))
	if err != nil {
		return
	}
	k, err = GenKeys(path)
	if err != nil {
		return
	}
	return
}

// LoadAgent gets the agent and signing key from the specified directory
func LoadAgent(path string) (agent AgentID, key *ecdsa.PrivateKey, err error) {
	a, err := readFile(path, AgentFileName)
	if err != nil {
		return
	}
	agent = AgentID(a)
	key, err = UnmarshalPrivateKey(path, PrivKeyFileName)
	return
}

// MarshalPublicKey stores a PublicKey to a serialized x509 format file
func MarshalPublicKey(path string, file string, key *ecdsa.PublicKey) error {
	k, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return err
	}
	err = writeFile(path, file, k)
	return err
}

// UnmarshalPublicKey loads a PublicKey from the serialized x509 format file
func UnmarshalPublicKey(path string, file string) (key *ecdsa.PublicKey, err error) {
	k, err := readFile(path, file)
	if err != nil {
		return nil, err
	}
	kk, err := x509.ParsePKIXPublicKey(k)
	key = kk.(*ecdsa.PublicKey)
	return key, err
}

// MarshalPrivateKey stores a PublicKey to a serialized x509 format file
func MarshalPrivateKey(path string, file string, key *ecdsa.PrivateKey) error {
	k, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	err = writeFile(path, file, k)
	return err
}

// UnmarshalPrivateKey loads a PublicKey from the serialized x509 format file
func UnmarshalPrivateKey(path string, file string) (key *ecdsa.PrivateKey, err error) {
	k, err := readFile(path, file)
	if err != nil {
		return nil, err
	}
	key, err = x509.ParseECPrivateKey(k)
	return key, err
}

// GenKeys creates a new pub/priv key pair and stores them at the given path.
func GenKeys(path string) (priv *ecdsa.PrivateKey, err error) {
	if fileExists(path + "/" + PrivKeyFileName) {
		return nil, errors.New("keys already exist")
	}
	priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		return
	}

	err = MarshalPrivateKey(path, PrivKeyFileName, priv)
	if err != nil {
		return
	}

	var pub *ecdsa.PublicKey
	pub = priv.Public().(*ecdsa.PublicKey)
	err = MarshalPublicKey(path, PubKeyFileName, pub)
	if err != nil {
		return
	}
	return
}
