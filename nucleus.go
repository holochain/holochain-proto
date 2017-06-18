// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

package holochain

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
	if err = n.h.node.StartProtocol(n.h, ActionProtocol); err != nil {
		return
	}
	return
}

type AppMsg struct {
	ZomeType string
	Body     string
}

// ActionReceiver handles messages on the action protocol
func ActionReceiver(h *Holochain, msg *Message) (response interface{}, err error) {
	dht := h.dht
	var a Action
	a, err = MakeActionFromMessage(msg)
	if err == nil {
		dht.dlog.Logf("ActionReceiver got %s: %v", a.Name(), msg)
		// N.B. a.Receive calls made to an Action whose values are NOT populated.
		// The Receive functions understand this and use the values from the message body
		// TODO, this indicates an architectural error, so fix!
		response, err = a.Receive(dht, msg)
	}
	return
}
