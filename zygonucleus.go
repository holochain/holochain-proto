// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// ZygoNucleus implements a zygomys use of the Nucleus interface

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

type ZygoNucleus struct {
	env        *zygo.Glisp
	interfaces []Interface
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

// GetInterface returns an Interface of the given name
func (z *ZygoNucleus) GetInterface(iface string) (i *Interface, err error) {
	for _, x := range z.interfaces {
		if x.Name == iface {
			i = &x
			break
		}
	}
	if i == nil {
		err = errors.New("couldn't find: " + iface)
	}
	return
}
func (z *ZygoNucleus) Interfaces() (i []Interface) {
	i = z.interfaces
	return
}

// Call calls the zygo function that was registered with expose
func (z *ZygoNucleus) Call(iface string, params interface{}) (result interface{}, err error) {
	i, err := z.GetInterface(iface)
	if err != nil {
		return
	}
	var code string
	switch i.Schema {
	case STRING:
		code = fmt.Sprintf(`(%s "%s")`, iface, strings.Replace(params.(string), "\"", "\\\"", -1))
	default:
		err = errors.New("params type not implemented")
		return
	}
	err = z.env.LoadString(code)
	if err != nil {
		return
	}
	result, err = z.env.Run()
	return
}

// These are the zygo implementations of the library functions that must available in
// all Nucleii implementations.
const (
	ZygoLibrary = `(def STRING 0) (def JSON 1)`
)

// Expose registers an interfaces defined in the DNA for calling by external clients
// (you should probably never need to call this function as it is called by the DNA's expose functions)
func (z *ZygoNucleus) expose(iface Interface) (err error) {
	z.interfaces = append(z.interfaces, iface)
	return
}

// NewZygoNucleus builds an zygo execution environment with user specified code
func NewZygoNucleus(code string) (v Nucleus, err error) {
	var z ZygoNucleus
	z.env = zygo.NewGlisp()
	z.env.AddFunction("version",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			return &zygo.SexpStr{S: Version}, nil
		})

	// use a closure so that the registered zygo function can call Expose on the correct ZygoNucleus obj
	z.env.AddFunction("expose",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			if len(args) != 2 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var i Interface

			switch t := args[0].(type) {
			case *zygo.SexpStr:
				i.Name = t.S
			default:
				return zygo.SexpNull,
					errors.New("1st argument of expose should be string")
			}

			switch t := args[1].(type) {
			case *zygo.SexpInt:
				i.Schema = InterfaceSchemaType(t.Val)
			default:
				return zygo.SexpNull,
					errors.New("1st argument of expose should be string")
			}

			err := z.expose(i)
			return zygo.SexpNull, err
		})

	err = z.env.LoadString(ZygoLibrary + code)
	if err != nil {
		err = errors.New("Zygomys error: " + err.Error())
		return
	}
	v = &z
	return
}
