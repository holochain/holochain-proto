// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Persister implements a persistence engine interface for storing data
// additionally it implements a bolt use of that interface

package holochain

import (
	_"errors"
	"github.com/boltdb/bolt"
	"time"
	"os"
)

const (
	IDMetaKey = "id"
	TopMetaKey = "top"

	MetaBucket = "M"
	HeaderBucket = "H"
	EntryBucket = "E"
)

type Persister interface {
	Open() error
	Init() error
	GetMeta(string) ([]byte,error)
	PutMeta(key string,value []byte) (err error)
	Get(hash Hash,getEntry bool) (header Header,entry interface{},err error)
	Remove() error
}

type BoltPersister struct {
	path string
	db *bolt.DB
}

// Open opens the data store
func (bp *BoltPersister) Open() (err error) {
	bp.db, err = bolt.Open(bp.path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {return}
	return
}

// Init opens the store (if it isn't already open) and initializes buckets
func (bp *BoltPersister) Init() (err error) {
	if bp.db == nil {err = bp.Open()}
	if err != nil {return}

	defer func() {if err !=nil {bp.db.Close();bp.db = nil}}()
	var initialized bool
	err = bp.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(MetaBucket))
		initialized = b != nil
		return nil
	})
	if err != nil {return}
	if !initialized {
		err = bp.db.Update(func(tx *bolt.Tx) (err error) {
			_, err = tx.CreateBucketIfNotExists([]byte(EntryBucket))
			if err != nil {return}
			_, err = tx.CreateBucketIfNotExists([]byte(HeaderBucket))
			if err != nil {return}
			_, err = tx.CreateBucketIfNotExists([]byte(MetaBucket))
			return
		})
	}

	return
}

// Get returns a header, and (optionally) it's entry if getEntry is true
func (bp *BoltPersister) Get(hash Hash,getEntry bool) (header Header,entry interface{},err error){
	err = bp.db.View(func(tx *bolt.Tx) error {
		hb := tx.Bucket([]byte(HeaderBucket))
		eb := tx.Bucket([]byte(EntryBucket))
		header,entry,err = get(hb,eb,hash[:],getEntry)
		return err
	})
	return
}

// GetMeta returns meta data
func (bp *BoltPersister) GetMeta(key string) (data []byte,err error) {
	err = bp.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(MetaBucket))
		data = b.Get([]byte(key))
		return nil
	})
	return
}

// PutMeta sets meta data
func (bp *BoltPersister) PutMeta(key string,value []byte) (err error) {
	err = bp.db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(MetaBucket))
		err = b.Put([]byte(key),value)
		return err
	})
	return
}

// Remove deletes all data in the datastore
func (bp *BoltPersister) Remove() (err error) {
	os.Remove(bp.path)
	bp.db = nil
	return nil
}


// NewBoltPersister returns a Bolt implementation of the Persister type
func NewBoltPersister(path string) (p Persister) {
	var bp BoltPersister
	bp.path = path
	p = &bp
	return
}

// DB returns the bolt db to give clients direct accesses to the bolt store
func (bp *BoltPersister) DB()  *bolt.DB {
	return bp.db
}
