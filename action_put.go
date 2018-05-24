package holochain

import (
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
)

//------------------------------------------------------------
// Put

type ActionPut struct {
	entryType string
	entry     Entry
	header    *Header
}

func NewPutAction(entryType string, entry Entry, header *Header) *ActionPut {
	a := ActionPut{entryType: entryType, entry: entry, header: header}
	return &a
}

func (a *ActionPut) Name() string {
	return "put"
}

func (a *ActionPut) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	err = sysValidateEntry(h, def, a.entry, pkg)
	return
}

func (a *ActionPut) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	// check to see if we are already holding this hash
	var status int
	_, _, _, status, err = dht.Get(t.EntryHash, StatusAny, GetMaskEntryType) //TODO should be a getmask for just Status
	if err == nil {
		holdResp, err = dht.MakeHoldResp(msg, status)
		response = *holdResp
		return
	}
	if err != ErrHashNotFound {
		return
	}

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_PUT_REQUEST, t.EntryHash, func(resp ValidateResponse) error {
		a := NewPutAction(resp.Type, &resp.Entry, &resp.Header)
		_, err := dht.h.ValidateAction(a, a.entryType, &resp.Package, []peer.ID{msg.From})

		var status int
		if err != nil {
			dht.dlog.Logf("Put %v rejected: %v", t.EntryHash, err)
			status = StatusRejected
		} else {
			status = StatusLive
		}
		entry := resp.Entry
		var b []byte
		b, err = entry.Marshal()
		if err == nil {
			err = dht.Put(msg, resp.Type, t.EntryHash, msg.From, b, status)
		}
		if err == nil {
			holdResp, err = dht.MakeHoldResp(msg, status)
		}
		return err
	})

	r := dht.h.RedundancyFactor()
	if r == 0 {
		r = CloserPeerCount
	}

	closest := dht.h.node.betterPeersForHash(&t.EntryHash, msg.From, true, r)
	if len(closest) > 0 {
		err = nil
		resp := CloserPeersResp{}
		resp.CloserPeers = dht.h.node.peers2PeerInfos(closest)
		response = resp
		return
	} else {
		if holdResp != nil {
			response = *holdResp
		}
	}
	return
}

func (a *ActionPut) CheckValidationRequest(def *EntryDef) (err error) {
	return
}
