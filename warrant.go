// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// warrant an interface for signed claims that can be cryptographically verified and implementation of various warrants

package holochain

import (
	"errors"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
)

const (
	SelfRevocationType = iota
)

// Warrant abstracts the notion of a multi-party cryptographically verifiable signed claim
// the meaning of the warrant is understood by the warrant name an/or by properties contained in it
type Warrant interface {

	// Int returns the warrant type
	Type() int

	// Parties returns the hashes of the public keys of the signers of the warrant
	Parties() ([]Hash, error)

	// Verify confirms that the content of a warrant is valid and has been signed by the
	// the parties in it.  Requires a Holochain object for context, returns nil if it
	// verfies or an error
	Verify(h *Holochain) error

	// Property returns a value of a property attested to by the warrant
	// returns a WarrantPropertyNotFoundErr if the warrant doesn't have that property
	Property(key string) (value interface{}, err error)

	// Encode marshals the warrant into bytes for sending over the wire
	Encode() (data []byte, err error)

	// Decode unmarshals a warrant from bytes
	Decode(data []byte) (err error)
}

var WarrantPropertyNotFoundErr = errors.New("warrant property not found")
var UnknownWarrantTypeErr = errors.New("unknown warrant type")

// SelfRevocationWarrant warrants that the first party revoked its own key in favor of the second
type SelfRevocationWarrant struct {
	Revocation SelfRevocation
}

func NewSelfRevocationWarrant(revocation *SelfRevocation) (wP *SelfRevocationWarrant, err error) {
	w := SelfRevocationWarrant{Revocation: *revocation}
	wP = &w
	return
}

func DecodeWarrant(warrantType int, data []byte) (w Warrant, err error) {
	switch warrantType {
	case SelfRevocationType:
		w = &SelfRevocationWarrant{}
		err = w.Decode(data)
	default:
		err = UnknownWarrantTypeErr
	}
	return
}

func (w *SelfRevocationWarrant) Type() int {
	return SelfRevocationType
}

func (w *SelfRevocationWarrant) Parties() (parties []Hash, err error) {
	var oldPubKey, newPubKey ic.PubKey
	oldPubKey, err = w.Revocation.getOldKey()
	if err != nil {
		return
	}
	newPubKey, err = w.Revocation.getNewKey()
	if err != nil {
		return
	}
	var ID peer.ID
	ID, err = peer.IDFromPublicKey(oldPubKey)
	if err != nil {
		return
	}
	var oldH, newH Hash
	oldH, err = NewHash(peer.IDB58Encode(ID))
	if err != nil {
		return
	}
	ID, err = peer.IDFromPublicKey(newPubKey)
	if err != nil {
		return
	}
	newH, err = NewHash(peer.IDB58Encode(ID))
	if err != nil {
		return
	}
	parties = append(parties, oldH, newH)

	return
}

func (w *SelfRevocationWarrant) Verify(h *Holochain) (err error) {
	// check that the revocation itself verifies
	err = w.Revocation.Verify()
	if err != nil {
		return
	}
	// also check that old and new keys appear as they should in the DHT

	var parties []Hash
	parties, err = w.Parties()
	if err != nil {
		return
	}

	var data []byte
	data, _, _, _, err = h.dht.get(parties[0], StatusDefault, GetMaskDefault)
	if err != ErrHashModified {
		err = errors.New("expected old key to be modified on DHT")
		return
	} else {
		err = nil
	}
	if string(data) != parties[1].String() {
		err = errors.New("expected old key to point to new key on DHT")
	}

	return
}

func (w *SelfRevocationWarrant) Property(key string) (value interface{}, err error) {
	data := w.Revocation.Data
	if key == "payload" {
		l := int(data[0])
		value = data[l*2+1 : len(data)]
		return
	}
	err = WarrantPropertyNotFoundErr
	return
}

func (w *SelfRevocationWarrant) Encode() (data []byte, err error) {
	data, err = w.Revocation.Marshal()
	return
}

func (w *SelfRevocationWarrant) Decode(data []byte) (err error) {
	err = w.Revocation.Unmarshal(data)
	return
}
