// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

package holochain

import (
	"fmt"
)

// Nucleus encapsulates Application parts: Ribosomes to run code in Zomes, plus application
// validation and direct message passing protocols
type Nucleus struct {
	h    *Holochain
	alog *Logger // the app logger

}

// NewNucleus creates a new Nucleus structure
func NewNucleus(h *Holochain) *Nucleus {
	nucleus := Nucleus{
		h:    h,
		alog: &h.config.Loggers.App,
	}
	return &nucleus
}

func (n *Nucleus) Start() (err error) {
	if err = n.h.node.StartProtocol(n.h, ValidateProtocol); err != nil {
		return
	}
	if err = n.h.node.StartProtocol(n.h, AppProtocol); err != nil {
		return
	}
	return
}

type AppMsg struct {
	ZomeType string
	Body     string
}

// AppReceiver handles messages on the application protocol
func AppReceiver(h *Holochain, msg *Message) (response interface{}, err error) {
	switch msg.Type {
	case APP_MESSAGE:
		h.config.Loggers.App.Logf("got app message: %v", msg)
		switch t := msg.Body.(type) {
		case AppMsg:
			var r Ribosome
			r, _, err = h.MakeRibosome(t.ZomeType)
			if err != nil {
				return
			}
			rsp := AppMsg{ZomeType: t.ZomeType}
			rsp.Body, err = r.Receive(t.Body)
			if err == nil {
				response = rsp
			}
		default:
			err = fmt.Errorf("expected AppMsg got %T", t)
		}
	default:
		err = fmt.Errorf("message type %d not in holochain-app protocol", int(msg.Type))
	}
	return
}
