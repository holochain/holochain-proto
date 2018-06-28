package holochain

import (
	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
)

//------------------------------------------------------------
// Mod

type APIFnMod struct {
	action ActionMod
}

func (fn *APIFnMod) Name() string {
	return fn.action.Name()
}

func (fn *APIFnMod) Args() []Arg {
	return []Arg{{Name: "entryType", Type: StringArg}, {Name: "entry", Type: EntryArg}, {Name: "replaces", Type: HashArg}}
}

func (fn *APIFnMod) Call(h *Holochain) (response interface{}, err error) {
	a := &fn.action
	response, err = h.commitAndShare(a, a.replaces)
	return
}

type ActionMod struct {
	entryType string
	entry     Entry
	header    *Header
	replaces  Hash
}

func NewModAction(entryType string, entry Entry, replaces Hash) *ActionMod {
	a := ActionMod{entryType: entryType, entry: entry, replaces: replaces}
	return &a
}

func (a *ActionMod) Entry() Entry {
	return a.entry
}

func (a *ActionMod) EntryType() string {
	return a.entryType
}

func (a *ActionMod) Name() string {
	return "mod"
}

func (a *ActionMod) SetHeader(header *Header) {
	a.header = header
}

func (a *ActionMod) GetHeader() (header *Header) {
	return a.header
}

func (a *ActionMod) Share(h *Holochain, def *EntryDef) (err error) {
	if def.isSharingPublic() {
		// if it's a public entry send the DHT MOD & PUT messages
		// TODO handle errors better!!
		h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.header.EntryLink})
		h.dht.Change(a.replaces, MOD_REQUEST, HoldReq{RelatedHash: a.replaces, EntryHash: a.header.EntryLink})
	}
	return
}

func (a *ActionMod) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	switch def.Name {
	case DNAEntryType:
		err = ErrNotValidForDNAType
		return
	case HeadersEntryType:
		err = ErrNotValidForHeadersType
		return
	case DelEntryType:
		err = ErrNotValidForDelType
		return
	case KeyEntryType:
	case AgentEntryType:
	}

	if def.DataFormat == DataFormatLinks {
		err = ErrModInvalidForLinks
		return
	}

	if a.entry == nil {
		err = ErrNilEntryInvalid
		return
	}
	if a.header == nil {
		err = ErrModMissingHeader
		return
	}
	if a.replaces.String() == a.header.EntryLink.String() {
		err = ErrModReplacesHashNotDifferent
		return
	}
	// no need to check for virtual entries on the chain because they aren't there
	// currently the only virtual entry is the node id
	/*
		if !def.IsVirtualEntry() {
			var header *Header
			header, err = h.chain.GetEntryHeader(a.replaces)
			if err != nil {
				return
			}
			if header.Type != a.entryType {
				err = ErrEntryTypeMismatch
				return
			}
		}*/
	err = sysValidateEntry(h, def, a.entry, pkg)
	return
}

func (a *ActionMod) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	//var hashStatus int
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_MOD_REQUEST, t.EntryHash, func(resp ValidateResponse) error {
		a := NewModAction(resp.Type, &resp.Entry, t.RelatedHash)
		a.header = &resp.Header

		//@TODO what comes back from Validate Mod
		_, err = dht.h.ValidateAction(a, resp.Type, &resp.Package, []peer.ID{msg.From})
		if err != nil {
			// how do we record an invalid Mod?
			//@TODO store as REJECTED?
		} else {
			err = dht.Mod(msg, t.RelatedHash, t.EntryHash)
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

func (a *ActionMod) CheckValidationRequest(def *EntryDef) (err error) {
	return
}
