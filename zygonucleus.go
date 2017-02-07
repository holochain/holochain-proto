// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// ZygoNucleus implements a zygomys use of the Nucleus interface

package holochain

import (
	"errors"
	"fmt"
	zygo "github.com/glycerine/zygomys/repl"
)

const (
	ZygoSchemaType = "zygo"
)

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

// These are the zygo implementations of the library functions that must available in
// all Nucleii implementations.
const (
	ZygoLibrary = ``
)

func ZygoVersionFunction(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
	return &zygo.SexpStr{S: Version}, nil
}

func ZygoExposeFunction(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
	return zygo.SexpNull, nil
}

// NewZygoNucleus builds an zygo execution environment with user specified code
func NewZygoNucleus(code string) (v Nucleus, err error) {
	var z ZygoNucleus
	z.env = zygo.NewGlisp()
	z.env.AddFunction("version", ZygoVersionFunction)
	z.env.AddFunction("expose", ZygoExposeFunction)

	err = z.env.LoadString(ZygoLibrary + code)
	if err != nil {
		err = errors.New("Zygomys error: " + err.Error())
		return
	}
	v = &z
	return
}
