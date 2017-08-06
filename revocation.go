// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// interface for revoking keys

package holochain

import (
	"encoding/json"
	"errors"
	ic "github.com/libp2p/go-libp2p-crypto"
)

type Revocation interface {
	Verify() error
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

var SelfRevocationDoesNotVerify = errors.New("self revocation does not verify")

// SelfRevocation holds the old key being revoked and the new key, other revocation data and
// the two cryptographic signatures of that data by the two keys to confirm the revocation
type SelfRevocation struct {
	Data   []byte // concatination of key length, two marshaled keys, and revocation properties
	OldSig []byte // signature of oldnew by old key
	NewSig []byte // signature by oldnew new key
}

func NewSelfRevocation(old, new ic.PrivKey, payload []byte) (rP *SelfRevocation, err error) {
	oldPub := old.GetPublic()
	newPub := new.GetPublic()
	var oldPubBytes, newPubBytes, oldSig, newSig []byte
	oldPubBytes, err = ic.MarshalPublicKey(oldPub)
	if err != nil {
		return
	}
	newPubBytes, _ = ic.MarshalPublicKey(newPub)
	if err != nil {
		return
	}
	data := []byte{byte(len(oldPubBytes))}

	data = append(data, oldPubBytes...)
	data = append(data, newPubBytes...)
	data = append(data, payload...)

	oldSig, err = old.Sign(data)
	newSig, err = new.Sign(data)

	revocation := SelfRevocation{
		Data:   data,
		OldSig: oldSig,
		NewSig: newSig,
	}
	rP = &revocation
	return
}

func (r *SelfRevocation) getOldKey() (key ic.PubKey, err error) {
	l := int(r.Data[0])
	bytes := r.Data[1 : l+1]
	key, err = ic.UnmarshalPublicKey(bytes)
	return

}

func (r *SelfRevocation) getNewKey() (key ic.PubKey, err error) {
	l := int(r.Data[0])
	bytes := r.Data[l+1 : l*2+1]
	key, err = ic.UnmarshalPublicKey(bytes)
	return
}

func (r *SelfRevocation) Marshal() (data []byte, err error) {
	data, err = json.Marshal(r)
	return
}

func (r *SelfRevocation) Unmarshal(data []byte) (err error) {
	err = json.Unmarshal(data, r)
	return
}

// Verify confirms that a self-revocation is properly signed
func (r *SelfRevocation) Verify() (err error) {
	var oldPubKey, newPubKey ic.PubKey
	oldPubKey, err = r.getOldKey()
	if err != nil {
		return
	}
	var matches bool
	matches, err = oldPubKey.Verify(r.Data, r.OldSig)
	if err != nil {
		return err
	}
	if !matches {
		return SelfRevocationDoesNotVerify
	}
	newPubKey, err = r.getNewKey()

	if err != nil {
		return
	}
	matches, err = newPubKey.Verify(r.Data, r.NewSig)
	if err != nil {
		return err
	}
	if !matches {
		return SelfRevocationDoesNotVerify
	}
	return
}
