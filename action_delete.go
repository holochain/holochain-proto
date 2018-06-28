package holochain

import (
	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
)

//------------------------------------------------------------
// Del

type APIFnDel struct {
	action ActionDel
}

func (fn *APIFnDel) Name() string {
	return fn.action.Name()
}

func (fn *APIFnDel) Args() []Arg {
	return []Arg{{Name: "hash", Type: HashArg}, {Name: "message", Type: StringArg}}
}

func (fn *APIFnDel) Call(h *Holochain) (response interface{}, err error) {
	a := &fn.action
	response, err = h.commitAndShare(a, NullHash())
	return
}

type ActionDel struct {
	entry  DelEntry
	header *Header
}

func NewDelAction(entry DelEntry) *ActionDel {
	a := ActionDel{entry: entry}
	return &a
}

func (a *ActionDel) Name() string {
	return "del"
}

func (a *ActionDel) Entry() Entry {
	j, err := a.entry.ToJSON()
	if err != nil {
		panic(err)
	}
	return &GobEntry{C: j}
}

func (a *ActionDel) EntryType() string {
	return DelEntryType
}

func (a *ActionDel) SetHeader(header *Header) {
	a.header = header
}

func (a *ActionDel) GetHeader() (header *Header) {
	return a.header
}

func (a *ActionDel) Share(h *Holochain, def *EntryDef) (err error) {
	if def.isSharingPublic() {
		// if it's a public entry send the DHT DEL & PUT messages
		h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.header.EntryLink})
		h.dht.Change(a.entry.Hash, DEL_REQUEST, HoldReq{RelatedHash: a.entry.Hash, EntryHash: a.header.EntryLink})
	}
	return
}

func (a *ActionDel) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	if def != DelEntryDef {
		err = ErrEntryDefInvalid
		return
	}
	return
}

func (a *ActionDel) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_DEL_REQUEST, t.EntryHash, func(resp ValidateResponse) error {

		var delEntry DelEntry
		delEntry, err = DelEntryFromJSON(resp.Entry.Content().(string))

		a := NewDelAction(delEntry)
		//@TODO what comes back from Validate Del
		_, err = dht.h.ValidateAction(a, resp.Type, &resp.Package, []peer.ID{msg.From})
		if err != nil {
			// how do we record an invalid DEL?
			//@TODO store as REJECTED
		} else {
			err = dht.Del(msg, delEntry.Hash)
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

func (a *ActionDel) CheckValidationRequest(def *EntryDef) (err error) {
	return
}
