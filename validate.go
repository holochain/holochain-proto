// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// Chain validation protocol.  This protocol allows DHT nodes to request data so they can
// run validation on the puts and linkings they are asked to perform

package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ValidateQuery holds the data from a validation query on the Source protocol
type ValidateQuery struct {
	H Hash
}

// ValidateResponse holds the response to a VALIDATE_PUT_REQUEST
type ValidateResponse struct {
	Entry  GobEntry
	Header Header
	Type   string
}

// ValidateLinkResponse holds the response to a VALIDATE_LINK_REQUEST
type ValidateLinkResponse struct {
	LinkingType string
	Links       []Link
}

// ValidateDelResponse holds the response to a VALIDATE_DEL_REQUEST
type ValidateDelResponse struct {
	Type string
}

// ValidateModResponse holds the response to a VALIDATE_DEL_REQUEST
type ValidateModResponse struct {
	Type string
}

// ValidateReceiver handles messages on the Validate protocol
func ValidateReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	switch m.Type {
	case VALIDATE_PUT_REQUEST:
		h.dht.dlog.Logf("got validate put: %v", m)
		switch t := m.Body.(type) {
		case ValidateQuery:
			var r ValidateResponse
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
			response = r
			h.dht.dlog.Logf("responding with: %v (err=%v)", r, err)
		default:
			err = fmt.Errorf("expected ValidateQuery got %T", t)
		}
	case VALIDATE_MOD_REQUEST:
		h.dht.dlog.Logf("got validate mod: %v", m)
		switch t := m.Body.(type) {
		case ValidateQuery:
			var r ValidateModResponse
			_, r.Type, err = h.chain.GetEntry(t.H)
			if err != nil {
				return
			}
			response = r
			h.dht.dlog.Logf("responding with: %v (err=%v)", r, err)
		default:
			err = fmt.Errorf("expected ValidateQuery got %T", t)
		}

	case VALIDATE_DEL_REQUEST:
		h.dht.dlog.Logf("got validate del: %v", m)
		switch t := m.Body.(type) {
		case ValidateQuery:
			var r ValidateDelResponse
			_, r.Type, err = h.chain.GetEntry(t.H)
			if err != nil {
				return
			}
			response = r
			h.dht.dlog.Logf("responding with: %v (err=%v)", r, err)
		default:
			err = fmt.Errorf("expected ValidateQuery got %T", t)
		}
	case VALIDATE_LINK_REQUEST:
		h.dht.dlog.Logf("got validatelink: %v", m)
		switch t := m.Body.(type) {
		case ValidateQuery:
			var r ValidateLinkResponse
			var e Entry
			var et string
			e, et, err = h.chain.GetEntry(t.H)
			if err == nil {
				var d *EntryDef
				_, d, err = h.GetEntryDef(et)
				if err == nil {
					if d.DataFormat != DataFormatLinks {
						err = errors.New("hash not of a linking entry")
					} else {
						var le LinksEntry
						if err = json.Unmarshal([]byte(e.Content().(string)), &le); err == nil {

							r.LinkingType = et
							r.Links = le.Links
						}
					}
				}
			}
			response = r
			h.dht.dlog.Logf("responding with: %v (err=%v)", r, err)

		default:
			err = fmt.Errorf("expected ValidateQuery got %T", t)
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
