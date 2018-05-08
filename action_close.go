// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
)

//------------------------------------------------------------
// Close

type APIFnClose struct {
	action ActionClose
}

func (fn *APIFnClose) Name() string {
	return fn.action.Name()
}

func (fn *APIFnClose) Args() []Arg {
	return []Arg{{Name: "message", Type: StringArg}}
}

func (fn *APIFnClose) Call(h *Holochain) (response interface{}, err error) {
	a := &fn.action
	response, err = h.commitAndShare(a, NullHash())
	return
}

type ActionClose struct {
	entry  CloseEntry
	header *Header
}

func NewCloseAction(entry CloseEntry) *ActionClose {
	a := ActionClose{entry: entry}
	return &a
}

func (a *ActionClose) Name() string {
	return "close"
}

func (a *ActionClose) Entry() Entry {
	j, err := a.entry.ToJSON()
	if err != nil {
		panic(err)
	}
	return &GobEntry{C: j}
}

func (a *ActionClose) EntryType() string {
	return CloseEntryType
}

func (a *ActionClose) SetHeader(header *Header) {
	a.header = header
}

func (a *ActionClose) GetHeader() (header *Header) {
	return a.header
}

func (a *ActionClose) Share(h *Holochain, def *EntryDef) (err error) {
	if def.isSharingPublic() {
		h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.EntryLink})
		h.dht.Change(a.entry.Hash, CLOSE_REQUEST, HoldReq{RelatedHash: a.entry.Hash, EntryHash: a.header.EntryLink})
	}
	return
}

func (a *ActionClose) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	if def != CloseEntryDef {
		err = ErrEntryDefInvalid
		return
	}
	return
}

func (a *ActionClose) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_CLOSE_REQUEST, t.EntryHash, func(resp ValidateResponse) error {

		var closeEntry CloseEntry
		closeEntry, err = CloseEntryFromJSON(resp.Entry.Content().(string))

		a := NewCloseAction(closeEntry)
		//@TODO what comes back from Validate Close
		_, err = dht.h.ValidateAction(a, resp.Type, &resp.Package, []peer.ID{msg.From})
		if err != nil {
			// how do we record an invalid CLOSE?
			//@TODO store as REJECTED
		} else {
			err = dht.Close(msg, closeEntry.Hash)
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

func (a *ActionClose) CheckValidationRequest(def *EntryDef) (err error) {
	return
}
