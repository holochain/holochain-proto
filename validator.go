// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Validator implements a validation engine interface for chains and their entries
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

type ValidatorFactory func(code string) (Validator, error)

type Validator interface {
	Name() string
	ValidateEntry(entry interface{}) error
}

type ZygoValidator struct {
	env *zygo.Glisp
}

// Name returns the string value under which this validator is registered
func (z *ZygoValidator) Name() string { return ZygoSchemaType }

// ValidateEntry checks the contents of an entry against the validation rules
// this is the zgo implementation
func (z *ZygoValidator) ValidateEntry(entry interface{}) (err error) {
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

func NewZygoValidator(code string) (v Validator, err error) {
	var z ZygoValidator
	z.env = zygo.NewGlisp()
	err = z.env.LoadString(code)
	if err != nil {
		err = errors.New("Zygomys error: " + err.Error())
		return
	}
	v = &z
	return
}

var validatorFactories = make(map[string]ValidatorFactory)

// RegisterValidator sets up a Validator to be used by the CreateValidator function
func RegisterValidator(name string, factory ValidatorFactory) {
	if factory == nil {
		panic("Datastore factory %s does not exist." + name)
	}
	_, registered := validatorFactories[name]
	if registered {
		panic("Datastore factory %s already registered. " + name)
	}
	validatorFactories[name] = factory
}

// RegisterBultinValidators adds the built in validator types to the factory hash
func RegisterBultinValidators() {
	RegisterValidator(ZygoSchemaType, NewZygoValidator)
}

// CreateValidator returns a new Validator of the given type
func CreateValidator(schema string, code string) (Validator, error) {

	factory, ok := validatorFactories[schema]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available datastore factories for logging.
		available := make([]string, 0)
		for k, _ := range validatorFactories {
			available = append(available, k)
		}
		return nil, errors.New(fmt.Sprintf("Invalid validator name. Must be one of: %s", strings.Join(available, ", ")))
	}

	return factory(code)
}
