// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// implements a general way for recording capabilities that can be stored, confirmed and revoked
//
// Used by various parts of the system, like for api keys for bridging between apps, etc.

package holochain

import (
	"errors"
	"fmt"
	"github.com/tidwall/buntdb"
	"math/rand"
)

type Capability struct {
	Token string
	db    *buntdb.DB
	//Who list of public keys for whom this it valid
}

var CapabilityInvalidErr = errors.New("invalid capability")

func makeToken(capability string) (token string) {
	return fmt.Sprintf("%d", rand.Int63())
}

// NewCapability returns and registers a capability of a type, for a specific or anyone if who is nil
func NewCapability(db *buntdb.DB, capability string, who interface{}) (c *Capability, err error) {
	c = &Capability{db: db}
	c.Token = makeToken(capability)
	err = db.Update(func(tx *buntdb.Tx) error {
		Debugf("NewCapability: save token:%s\n", c.Token)
		_, _, err = tx.Set("tok:"+c.Token, capability, nil)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

// Validate checks to see if the token has been registered and returns the capability it represent
func (c *Capability) Validate(who interface{}) (capability string, err error) {
	err = c.db.View(func(tx *buntdb.Tx) (e error) {
		Debugf("Validate: get token:%s\n", c.Token)
		capability, e = tx.Get("tok:" + c.Token)
		if e == buntdb.ErrNotFound {
			e = CapabilityInvalidErr
		}
		return
	})
	return
}

// Revoke unregisters the capability for a peer
func (c *Capability) Revoke(who interface{}) (err error) {
	err = c.db.Update(func(tx *buntdb.Tx) (e error) {
		_, e = tx.Get("tok:" + c.Token)
		if e == buntdb.ErrNotFound {
			e = CapabilityInvalidErr
		} else if e == nil {
			_, e = tx.Delete("tok:" + c.Token)
		}
		return e
	})
	return
}
