// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Nucleus provides an interface for an execution environment interface for chains and their entries
// and factory code for creating nucleii instances

package holochain

import (
	"errors"
	"fmt"
	//peer "gx/ipfs/QmZcUPvPhD1Xvk6mwijYF8AfR3mG31S1YsEfHG4khrFPRr/go-libp2p-peer"
	"sort"
	"strings"
)

type NucleusFactory func(h *Holochain, code string) (Nucleus, error)

type InterfaceSchemaType int

const (
	STRING InterfaceSchemaType = iota
	JSON
)

const (
	ID_PROPERTY         = "_id"
	AGENT_ID_PROPERTY   = "_agent_id"
	AGENT_NAME_PROPERTY = "_agent_name"
)

// Interface holds the name and schema of an DNA exposed function
type Interface struct {
	Name   string
	Schema InterfaceSchemaType
}

// ValidationProps holds the properties passed to the application validation routine
// This includes the Headers and Sources
type ValidationProps struct {
	Sources []string // B58 encoded peer
	Headers []Header
	MetaTag string // if validating a putMeta this will have the meta type set
}

// Nucleus type abstracts the functions of code execution environments
type Nucleus interface {
	Type() string
	ValidateEntry(def *EntryDef, entry Entry, props *ValidationProps) error
	ChainGenesis() error
	expose(iface Interface) error
	Interfaces() (i []Interface)
	Call(iface string, params interface{}) (interface{}, error)
}

var nucleusFactories = make(map[string]NucleusFactory)

// InterfaceSchema returns a functions schema type
func InterfaceSchema(n Nucleus, name string) (InterfaceSchemaType, error) {
	i := n.Interfaces()
	for _, f := range i {
		if f.Name == name {
			return f.Schema, nil
		}
	}
	return -1, errors.New("function not found: " + name)
}

// RegisterNucleus sets up a Nucleus to be used by the CreateNucleus function
func RegisterNucleus(name string, factory NucleusFactory) {
	if factory == nil {
		panic("Nucleus factory for type %s does not exist." + name)
	}
	_, registered := nucleusFactories[name]
	if registered {
		panic("Nucleus factory for type %s already registered. " + name)
	}
	nucleusFactories[name] = factory
}

// RegisterBultinNucleii adds the built in nucleus types to the factory hash
func RegisterBultinNucleii() {
	RegisterNucleus(ZygoNucleusType, NewZygoNucleus)
	RegisterNucleus(JSNucleusType, NewJSNucleus)
}

// CreateNucleus returns a new Nucleus of the given type
func CreateNucleus(h *Holochain, nucleusType string, code string) (Nucleus, error) {

	factory, ok := nucleusFactories[nucleusType]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available nucleus factories for error.
		available := make([]string, 0)
		for k := range nucleusFactories {
			available = append(available, k)
		}
		sort.Strings(available)
		return nil, fmt.Errorf("Invalid nucleus name. Must be one of: %s", strings.Join(available, ", "))
	}

	return factory(h, code)
}
