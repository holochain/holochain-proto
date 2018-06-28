// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	. "github.com/HC-Interns/holochain-proto/hash"
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

func (action *ActionMigrate) Share(h *Holochain, def *EntryDef) (err error) {
	err = h.dht.Change(action.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: action.header.EntryLink})
	return
}

func (action *ActionMigrate) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	// correct entry def
	if def != MigrateEntryDef {
		err = ErrEntryDefInvalid
		return
	}
	// has a header
	if action.header == nil {
		err = ErrActionMissingHeader
		return
	}
	// entry is valid
	err = sysValidateEntry(h, def, action.Entry(), pkg)
	// @TODO should migration only be valid if peer ID is node owner?
	return
}

func (a *ActionMigrate) CheckValidationRequest(def *EntryDef) (err error) {
	// intentionally left blank ;)
	return
}

func (a *ActionMigrate) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	// this is always an error because there is no action message for migrate
	err = ErrActionReceiveInvalid
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
	return []Arg{{Name: "migrationType",
		Type: StringArg},
		{Name: "DNAHash",
			Type: HashArg},
		{Name: "Key",
			Type: HashArg},
		{Name: "data",
			Type: StringArg}}
}

func (fn *APIFnMigrate) Call(h *Holochain) (response interface{}, err error) {
	var hash Hash
	response, err = h.commitAndShare(&fn.action, hash)
	return
}
