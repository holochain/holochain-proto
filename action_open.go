// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	. "github.com/holochain/holochain-proto/hash"
)

//------------------------------------------------------------
// Open

type APIFnOpen struct {
	action ActionOpen
}

func (fn *APIFnOpen) Name() string {
	return fn.action.Name()
}

func (fn *APIFnOpen) Args() []Arg {
	return []Arg{{Name: "message", Type: StringArg}}
}

func (fn *APIFnOpen) Call(h *Holochain) (response interface{}, err error) {
	a := &fn.action
	response, err = h.commitAndShare(a, NullHash())
	return
}

type ActionOpen struct {
	entry  OpenEntry
	header *Header
}

func NewOpenAction(entry OpenEntry) *ActionOpen {
	a := ActionOpen{entry: entry}
	return &a
}

func (a *ActionOpen) Name() string {
	return "open"
}

func (a *ActionOpen) Entry() Entry {
	j, err := a.entry.ToJSON()
	if err != nil {
		panic(err)
	}
	return &GobEntry{C: j}
}

func (a *ActionOpen) EntryType() string {
	return OpenEntryType
}

func (a *ActionOpen) SetHeader(header *Header) {
	a.header = header
}

func (a *ActionOpen) GetHeader() (header *Header) {
	return a.header
}

func (a *ActionOpen) Share(h *Holochain, def *EntryDef) (err error) {
	if def.isSharingPublic() {
		h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.EntryLink})
		h.dht.Change(a.entry.Hash, OPEN_REQUEST, HoldReq{RelatedHash: a.entry.Hash, EntryHash: a.header.EntryLink})
	}
	return
}

func (a *ActionOpen) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	if def != OpenEntryDef {
		err = ErrEntryDefInvalid
		return
	}
	return
}

func (a *ActionOpen) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_OPEN_REQUEST, t.EntryHash, func(resp ValidateResponse) error {

		var openEntry OpenEntry
		openEntry, err = OpenEntryFromJSON(resp.Entry.Content().(string))

		a := NewOpenAction(openEntry)
		//@TODO what comes back from Validate Open
		_, err = dht.h.ValidateAction(a, resp.Type, &resp.Package, []peer.ID{msg.From})
		if err != nil {
			// how do we record an invalid OPEN?
			//@TODO store as REJECTED
		} else {
			err = dht.Open(msg, openEntry.Hash)
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

func (a *ActionOpen) CheckValidationRequest(def *EntryDef) (err error) {
	return
}
