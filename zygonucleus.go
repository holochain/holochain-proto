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
	ZygoNucleusType = "zygo"
)

type ZygoNucleus struct {
	env        *zygo.Glisp
	interfaces []Interface
	lastResult zygo.Sexp
}

// Name returns the string value under which this nucleus is registered
func (z *ZygoNucleus) Type() string { return ZygoNucleusType }

// ValidateEntry checks the contents of an entry against the validation rules
// this is the zgo implementation
func (z *ZygoNucleus) ValidateEntry(d *EntryDef, entry interface{}) (err error) {
	// @todo handle JSON if schema type is different
	var e string
	switch d.Schema {
	case "zygo":
		e = entry.(string)
	case "string":
		e = "\"" + sanitizeString(entry.(string)) + "\""
	default:
		err = errors.New("schema type not implemented: " + d.Schema)
	}
	err = z.env.LoadString(fmt.Sprintf(`(validate "%s" %s)`, d.Name, e))
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
		err = errors.New("couldn't find exposed function: " + iface)
	}
	return
}

func (z *ZygoNucleus) Interfaces() (i []Interface) {
	i = z.interfaces
	return
}

// sanatizeString makes sure all quotes are quoted
func sanitizeString(s string) string {
	return strings.Replace(s, "\"", "\\\"", -1)
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
		code = fmt.Sprintf(`(%s "%s")`, iface, sanitizeString(params.(string)))
	default:
		err = errors.New("params type not implemented")
		return
	}
	err = z.env.LoadString(code)
	if err != nil {
		return
	}
	result, err = z.env.Run()
	if err == nil {
		switch t := result.(type) {
		case *zygo.SexpStr:
			result = t.S
		case *zygo.SexpInt:
			result = t.Val
		//case *zygo.SexpNull:
		//	result = nil
		default:
			result = fmt.Sprintf("%v", result)
		}

	}
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
func NewZygoNucleus(code string) (n Nucleus, err error) {
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

	_, err = z.Run(ZygoLibrary + code)
	if err != nil {
		return
	}
	n = &z
	return
}

func (z *ZygoNucleus) Run(code string) (result zygo.Sexp, err error) {
	err = z.env.LoadString(code)
	if err != nil {
		err = errors.New("Zygomys load error: " + err.Error())
		return
	}
	result, err = z.env.Run()
	if err != nil {
		err = errors.New("Zygomys exec error: " + err.Error())
		return
	}
	z.lastResult = result
	return
}
