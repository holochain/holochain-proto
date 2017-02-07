// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Nucleus provides an interface for an execution environment interface for chains and their entries
// and factory code for creating nucleii instances

package holochain

import (
	"errors"
	"fmt"
	"strings"
)

type NucleusFactory func(code string) (Nucleus, error)

type Nucleus interface {
	Name() string
	ValidateEntry(entry interface{}) error
}

var nucleusFactories = make(map[string]NucleusFactory)

// RegisterNucleus sets up a Nucleus to be used by the CreateNucleus function
func RegisterNucleus(name string, factory NucleusFactory) {
	if factory == nil {
		panic("Nucleus factory %s does not exist." + name)
	}
	_, registered := nucleusFactories[name]
	if registered {
		panic("Nucleus factory %s already registered. " + name)
	}
	nucleusFactories[name] = factory
}

// RegisterBultinNucleii adds the built in nucleus types to the factory hash
func RegisterBultinNucleii() {
	RegisterNucleus(ZygoSchemaType, NewZygoNucleus)
}

// CreateNucleus returns a new Nucleus of the given type
func CreateNucleus(schema string, code string) (Nucleus, error) {

	factory, ok := nucleusFactories[schema]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available datastore factories for logging.
		available := make([]string, 0)
		for k, _ := range nucleusFactories {
			available = append(available, k)
		}
		return nil, errors.New(fmt.Sprintf("Invalid nucleus name. Must be one of: %s", strings.Join(available, ", ")))
	}

	return factory(code)
}
