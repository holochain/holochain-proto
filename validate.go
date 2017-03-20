// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// Chain validation protocol.  This protocol allows DHT nodes to request data so they can
// run validation on the puts and putmetas they are asked to perform

package holochain

import (
	"errors"
	"fmt"
)

// ValidateQuery holds the data from a validation query on the Source protocol
type ValidateQuery struct {
	H Hash
}

// ValidateResponse holds the response to a ValidateQuery
type ValidateResponse struct {
	Entry  Entry
	Header Header
	Type   string
}

// ValidateMetaResponse holds the response to a ValidateQuery
type ValidateMetaResponse struct {
	Entry Entry
	Type  string
	Tag   string
}

// ValidateReceiver handles messages on the Validate protocol
func ValidateReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	switch m.Type {
	case VALIDATE_REQUEST:
		h.dht.dlog.Logf("got validate: %v", m)
		switch t := m.Body.(type) {
		case ValidateQuery:
			var r ValidateResponse
			r.Entry, r.Type, err = h.chain.GetEntry(t.H)
			if err != nil {
				return
			}
			var hd *Header
			hd, err = h.chain.GetEntryHeader(t.H)
			if err != nil {
				return
			}
			r.Header = *hd
			response = &r
			h.dht.dlog.Logf("responding with: %v (err=%v)", r, err)
		default:
			err = errors.New("expected ValidateQuery")
		}
	case VALIDATEMETA_REQUEST:
		h.dht.dlog.Logf("got validatemeta: %v", m)
		switch t := m.Body.(type) {
		case ValidateQuery:
			var r ValidateMetaResponse
			var e Entry
			var et string
			e, et, err = h.chain.GetEntry(t.H)
			if err == nil {
				if et != MetaEntryType {
					err = errors.New("hash not of meta entry")
				} else {
					me := e.Content().(MetaEntry)
					r.Tag = me.Tag
					r.Entry, r.Type, err = h.chain.GetEntry(me.M)
				}
			}
			response = &r
			h.dht.dlog.Logf("responding with: %v (err=%v)", r, err)

		default:
			err = errors.New("expected ValidateQuery")
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
