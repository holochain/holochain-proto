// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// Chain validation protocol.  This protocol allows DHT nodes to request data so they can
// run validation on the puts and linkings they are asked to perform

package holochain

import (
	"errors"
	"fmt"
)

// ValidateQuery holds the data from a validation query on the Source protocol
type ValidateQuery struct {
	H Hash
}

// ValidateResponse holds the response to committing validates (PUT/MOD/DEL)
type ValidateResponse struct {
	Type   string
	Header Header
	Entry  GobEntry
}

func makeValidateResponse(h *Holochain, m *Message) (r ValidateResponse, err error) {
	switch t := m.Body.(type) {
	case ValidateQuery:
		var entry Entry
		entry, r.Type, err = h.chain.GetEntry(t.H)
		if err != nil {
			return
		}
		r.Entry = *(entry.(*GobEntry))
		var hd *Header
		hd, err = h.chain.GetEntryHeader(t.H)
		if err != nil {
			return
		}
		r.Header = *hd
		h.dht.dlog.Logf("responding with: %v (err=%v)", r, err)
	default:
		err = fmt.Errorf("expected ValidateQuery got %T", t)
	}
	return
}

// ValidateReceiver handles messages on the Validate protocol
func ValidateReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	switch m.Type {
	case VALIDATE_PUT_REQUEST:
		h.dht.dlog.Logf("got validate put: %v", m)
		response, err = makeValidateResponse(h, m)
	case VALIDATE_MOD_REQUEST:
		h.dht.dlog.Logf("got validate mod: %v", m)
		response, err = makeValidateResponse(h, m)
	case VALIDATE_DEL_REQUEST:
		h.dht.dlog.Logf("got validate del: %v", m)
		response, err = makeValidateResponse(h, m)
	case VALIDATE_LINK_REQUEST:
		h.dht.dlog.Logf("got validatelink: %v", m)
		var r ValidateResponse
		r, err = makeValidateResponse(h, m)
		if err == nil {
			response = r
			var def *EntryDef
			_, def, err = h.GetEntryDef(r.Type)
			if err == nil && def.DataFormat != DataFormatLinks {
				err = errors.New("hash not of a linking entry")
			}
		}
	default:
		err = fmt.Errorf("message type %d not in holochain-validate protocol", int(m.Type))
	}
	return
}

// StartValidate initiates listening for Validate protocol messages on the node
func (node *Node) StartValidate(h *Holochain) (err error) {
	return node.StartProtocol(h, ValidateProtocol)
}
