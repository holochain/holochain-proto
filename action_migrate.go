// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	. "github.com/holochain/holochain-proto/hash"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
)

//------------------------------------------------------------
// Migrate

type APIFnMigrate struct {
	action ActionMigrate
}

func (fn *APIFnMigrate) Name() string {
	return fn.action.Name()
}

func (fn *APIFnMigrate) Args() []Arg {
	return []Arg{{Name: "message", Type: StringArg}}
}

func (fn *APIFnMigrate) Call(h *Holochain) (response interface{}, err error) {
	a := &fn.action
	response, err = h.commitAndShare(a, NullHash())
	return
}

type ActionMigrate struct {
	entry  MigrateEntry
	header *Header
}

func NewMigrateAction(entry MigrateEntry) *ActionMigrate {
	a := ActionMigrate{entry: entry}
	return &a
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
	if def.isSharingPublic() {
		h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.header.EntryLink})
		h.dht.Change(a.entry.Hash, MIGRATE_REQUEST, HoldReq{RelatedHash: a.entry.Hash, EntryHash: a.header.EntryLink})
	}
	return
}

func (a *ActionMigrate) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	if def != MigrateEntryDef {
		err = ErrEntryDefInvalid
		return
	}
	return
}

func (a *ActionMigrate) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_MIGRATE_REQUEST, t.EntryHash, func(resp ValidateResponse) error {

		var migrateEntry MigrateEntry
		migrateEntry, err = MigrateEntryFromJSON(resp.Entry.Content().(string))

		a := NewMigrateAction(migrateEntry)
		// @TODO what comes back from Validate Migrate
		// https://github.com/holochain/holochain-proto/issues/710
		_, err = dht.h.ValidateAction(a, resp.Type, &resp.Package, []peer.ID{msg.From})
		if err != nil {
			// how do we record an invalid MIGRATE?
			// @TODO store as REJECTED
			// https://github.com/holochain/holochain-proto/issues/711
		} else {
			err = dht.Migrate(msg, migrateEntry.Hash)
			if err == nil {
				holdResp, err = dht.MakeHoldResp(msg, StatusLive)
			}
		}
		return err
	})
	if holdResp != nil {
		response = *holdResp
	}
	return
}

func (a *ActionMigrate) CheckValidationRequest(def *EntryDef) (err error) {
	return
}
