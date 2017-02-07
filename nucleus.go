// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Nucleus implements an execution environment interface for chains and their entries
// additionally it implements a zygomys use of that interface

package holochain

import (
	"errors"
	"fmt"
	zygo "github.com/glycerine/zygomys/repl"
	"strings"
)

const (
	ZygoSchemaType = "zygo"
)

type NucleusFactory func(code string) (Nucleus, error)

type Nucleus interface {
	Name() string
	ValidateEntry(entry interface{}) error
}

type ZygoNucleus struct {
	env *zygo.Glisp
}

// Name returns the string value under which this nucleus is registered
func (z *ZygoNucleus) Name() string { return ZygoSchemaType }

// ValidateEntry checks the contents of an entry against the validation rules
// this is the zgo implementation
func (z *ZygoNucleus) ValidateEntry(entry interface{}) (err error) {
	e := entry.(string)
	err = z.env.LoadString("(validateEntry " + e + ")")
	if err != nil {
		return
	}
	result, err := z.env.Run()
	switch result.(type) {
	case *zygo.SexpBool:
		r := result.(*zygo.SexpBool).Val
		if !r {
			err = errors.New("Invalid entry:" + e)
		}
	default:
		err = errors.New("Unexpected result: " + fmt.Sprintf("%v", result))
	}
	return
}

func NewZygoNucleus(code string) (v Nucleus, err error) {
	var z ZygoNucleus
	z.env = zygo.NewGlisp()
	err = z.env.LoadString(code)
	if err != nil {
		err = errors.New("Zygomys error: " + err.Error())
		return
	}
	v = &z
	return
}

var nucleusFactories = make(map[string]NucleusFactory)

// RegisterNucleus sets up a Nucleus to be used by the CreateNucleus function
func RegisterNucleus(name string, factory NucleusFactory) {
	if factory == nil {
		panic("Datastore factory %s does not exist." + name)
	}
	_, registered := nucleusFactories[name]
	if registered {
		panic("Datastore factory %s already registered. " + name)
	}
	nucleusFactories[name] = factory
}

// RegisterBultinNucleii adds the built in validator types to the factory hash
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
		return nil, errors.New(fmt.Sprintf("Invalid validator name. Must be one of: %s", strings.Join(available, ", ")))
	}

	return factory(code)
}
