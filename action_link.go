package holochain

import (
	"errors"
	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
)

//------------------------------------------------------------
// Link

type ActionLink struct {
	entryType      string
	links          []Link
	validationBase Hash
}

func NewLinkAction(entryType string, links []Link) *ActionLink {
	a := ActionLink{entryType: entryType, links: links}
	return &a
}

func (a *ActionLink) Name() string {
	return "link"
}

func (a *ActionLink) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	if def.DataFormat != DataFormatLinks {
		err = errors.New("action only valid for links entry type")
	}
	//@TODO what sys level links validation?  That they are all valid hash format for the DNA?
	return
}

func (a *ActionLink) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_LINK_REQUEST, t.EntryHash, func(resp ValidateResponse) error {
		var le LinksEntry
		le, err = LinksEntryFromJSON(resp.Entry.Content().(string))
		if err != nil {
			return err
		}

		a := NewLinkAction(resp.Type, le.Links)
		a.validationBase = t.RelatedHash
		_, err = dht.h.ValidateAction(a, a.entryType, &resp.Package, []peer.ID{msg.From})
		//@TODO this is "one bad apple spoils the lot" because the app
		// has no way to tell us not to link certain of the links.
		// we need to extend the return value of the app to be able to
		// have it reject a subset of the links.
		if err != nil {
			// how do we record an invalid linking?
			//@TODO store as REJECTED
		} else {
			base := t.RelatedHash.String()
			for _, l := range le.Links {
				if base == l.Base {
					if l.LinkAction == DelLinkAction {
						err = dht.DelLink(msg, base, l.Link, l.Tag)
					} else {
						err = dht.PutLink(msg, base, l.Link, l.Tag)
					}
				}
			}
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

func (a *ActionLink) CheckValidationRequest(def *EntryDef) (err error) {
	if def.DataFormat != DataFormatLinks {
		err = errors.New("hash not of a linking entry")
	}
	return
}
