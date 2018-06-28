package holochain

import (
	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
)

//------------------------------------------------------------
// Commit

type APIFnCommit struct {
	action ActionCommit
}

func (fn *APIFnCommit) Name() string {
	return fn.action.Name()
}

func (fn *APIFnCommit) Args() []Arg {
	return []Arg{{Name: "entryType", Type: StringArg}, {Name: "entry", Type: EntryArg}}
}

func (fn *APIFnCommit) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.commitAndShare(&fn.action, NullHash())
	return
}

func (fn *APIFnCommit) SetAction(a *ActionCommit) {
	fn.action = *a
}

type ActionCommit struct {
	entryType string
	entry     Entry
	header    *Header
}

func NewCommitAction(entryType string, entry Entry) *ActionCommit {
	a := ActionCommit{entryType: entryType, entry: entry}
	return &a
}

func (a *ActionCommit) Entry() Entry {
	return a.entry
}

func (a *ActionCommit) EntryType() string {
	return a.entryType
}

func (a *ActionCommit) Name() string {
	return "commit"
}

func (a *ActionCommit) SetHeader(header *Header) {
	a.header = header
}

func (a *ActionCommit) GetHeader() (header *Header) {
	return a.header
}

func (a *ActionCommit) Share(h *Holochain, def *EntryDef) (err error) {
	if def.DataFormat == DataFormatLinks {
		// if this is a Link entry we have to send the DHT Link message
		var le LinksEntry
		entryStr := a.entry.Content().(string)
		le, err = LinksEntryFromJSON(entryStr)
		if err != nil {
			return
		}

		bases := make(map[string]bool)
		for _, l := range le.Links {
			_, exists := bases[l.Base]
			if !exists {
				b, _ := NewHash(l.Base)
				h.dht.Change(b, LINK_REQUEST, HoldReq{RelatedHash: b, EntryHash: a.header.EntryLink})
				//TODO errors from the send??
				bases[l.Base] = true
			}
		}
	}
	if def.isSharingPublic() {
		// otherwise we check to see if it's a public entry and if so send the DHT put message
		err = h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.header.EntryLink})
		if err == ErrEmptyRoutingTable {
			// will still have committed locally and can gossip later
			err = nil
		}
	}
	return
}

func (a *ActionCommit) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	err = sysValidateEntry(h, def, a.entry, pkg)
	return
}

func (a *ActionCommit) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	err = NonDHTAction
	return
}

func (a *ActionCommit) CheckValidationRequest(def *EntryDef) (err error) {
	return
}
