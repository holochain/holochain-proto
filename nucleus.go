// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

package holochain

import (
	"fmt"
	"github.com/google/uuid"
	peer "github.com/libp2p/go-libp2p-peer"
)

type DNA struct {
	Version                   int
	UUID                      uuid.UUID
	Name                      string
	Properties                map[string]string
	PropertiesSchema          string
	BasedOn                   Hash // references hash of another holochain that these schemas and code are derived from
	RequiresVersion           int
	DHTConfig                 DHTConfig
	Progenitor                Progenitor
	Zomes                     []Zome
	propertiesSchemaValidator SchemaValidator
}

func (dna *DNA) check() (err error) {
	if dna.RequiresVersion > Version {
		err = fmt.Errorf("Chain requires Holochain version %d", dna.RequiresVersion)
	}
	return
}

// Nucleus encapsulates Application parts: Ribosomes to run code in Zomes, plus application
// validation and direct message passing protocols
type Nucleus struct {
	dna  *DNA
	h    *Holochain
	alog *Logger // the app logger
}

func (n *Nucleus) DNA() (dna *DNA) {
	return n.dna
}

// NewNucleus creates a new Nucleus structure
func NewNucleus(h *Holochain, dna *DNA) *Nucleus {
	nucleus := Nucleus{
		dna:  dna,
		h:    h,
		alog: &h.config.Loggers.App,
	}
	return &nucleus
}

func (n *Nucleus) RunGenesis() {
	// run the init functions of each zome
	for _, zome := range n.dna.Zomes {
		ribosome, err := zome.MakeRibosome(n.h)
		if err == nil {
			err = ribosome.ChainGenesis()
			if err != nil {
				err = fmt.Errorf("In '%s' zome: %s", zome.Name, err.Error())
				return
			}
		}
	}
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
			rsp.Body, err = r.Receive(peer.IDB58Encode(msg.From), t.Body)
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
