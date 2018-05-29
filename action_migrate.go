// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	. "github.com/holochain/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
)

//------------------------------------------------------------
// Migrate Action

type ActionMigrate struct {
	entry  MigrateEntry
	header *Header
}

func (a *ActionMigrate) Name() string {
	return "migrate"
}

func (a *ActionMigrate) Entry() Entry {
	j, err := a.entry.ToJSON()
	if err != nil {
		panic(err)
	}
	return &GobEntry{C: j}
}

func (a *ActionMigrate) EntryType() string {
	return MigrateEntryType
}

func (a *ActionMigrate) SetHeader(header *Header) {
	a.header = header
}

func (a *ActionMigrate) GetHeader() (header *Header) {
	return a.header
}

func (a *ActionMigrate) Share(h *Holochain, def *EntryDef) (err error) {
	h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.header.EntryLink})
	return
}

func (a *ActionMigrate) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	if def != MigrateEntryDef {
		err = ErrEntryDefInvalid
		return
	}
	// @TODO call sysValidation for entry
	return
}

func (a *ActionMigrate) CheckValidationRequest(def *EntryDef) (err error) {
	// intentionally left blank ;)
	return
}

func (a *ActionMigrate) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	// @TODO this is an error because there is no action message, so return some error
	return
}

//------------------------------------------------------------
// Migrate API fn

type APIFnMigrate struct {
	action ActionMigrate
}

func (fn *APIFnMigrate) Name() string {
	return fn.action.Name()
}

func (fn *APIFnMigrate) Args() []Arg {
	// @TODO name of args
	// ID -> Key
	// DNA -> DNAHash
	return []Arg{{Name: "migrationType", Type: StringArg}, {Name: "DNA", Type: HashArg}, {Name: "ID", Type: HashArg}, {Name: "data", Type: StringArg}}
}

func (fn *APIFnMigrate) Call(h *Holochain) (response interface{}, err error) {
	action := &fn.action
	var hash Hash
	response, err = h.commitAndShare(action, hash)
	return
}
