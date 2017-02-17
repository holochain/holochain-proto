// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// ZygoNucleus implements a zygomys use of the Nucleus interface

package holochain

import (
	"errors"
	"fmt"
	zygo "github.com/glycerine/zygomys/repl"
	"math"
	"strconv"
	"strings"
	"time"
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
	switch d.DataFormat {
	case "zygo":
		e = entry.(string)
	case "string":
		e = "\"" + sanitizeString(entry.(string)) + "\""
	case "JSON":
		e = fmt.Sprintf(`(unjson (raw "%s"))`, sanitizeString(entry.(string)))
	default:
		err = errors.New("data format not implemented: " + d.DataFormat)
		return
	}
	err = z.env.LoadString(fmt.Sprintf(`(validate "%s" %s)`, d.Name, e))
	if err != nil {
		return
	}
	result, err := z.env.Run()
	if err != nil {
		err = errors.New(fmt.Sprintf("Error executing validate: %v", err))
		return
	}
	switch result.(type) {
	case *zygo.SexpBool:
		r := result.(*zygo.SexpBool).Val
		if !r {
			err = errors.New(fmt.Sprintf("Invalid entry: %v", entry))
		}
	case *zygo.SexpSentinel:
		err = errors.New("validate should return boolean, got nil")

	default:
		err = errors.New("validate should return boolean, got: " + fmt.Sprintf("%v", result))
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
	case JSON:
		code = fmt.Sprintf(`(json (%s (unjson (raw "%s"))))`, iface, sanitizeString(params.(string)))
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
		switch i.Schema {
		case STRING:
			switch t := result.(type) {
			case *zygo.SexpStr:
				result = t.S
			case *zygo.SexpInt:
				result = fmt.Sprintf("%d", t.Val)
			case *zygo.SexpRaw:
				result = string(t.Val)
			default:
				result = fmt.Sprintf("%v", result)
			}
		case JSON:
			// type should always be SexpRaw
			switch t := result.(type) {
			case *zygo.SexpRaw:
				result = t.Val
			default:
				err = errors.New("expected SexpRaw return type!")
			}
		}

	}
	return
}

// These are the zygo implementations of the library functions that must available in
// all Nucleii implementations.
const (
	ZygoLibrary = `(def STRING 0) (def JSON 1)`
)

// expose registers an interfaces defined in the DNA for calling by external clients
// (you should probably never need to call this function as it is called by the DNA's expose functions)
func (z *ZygoNucleus) expose(iface Interface) (err error) {
	z.interfaces = append(z.interfaces, iface)
	return
}

// put exposes DHTPut to holochain apps.
func (z *ZygoNucleus) put(env *zygo.Glisp, h *Holochain, params *zygo.SexpHash) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}

	var put_result string
	hash, err := params.HashGet(env, env.MakeSymbol("hsh"))
	if err != nil {
		return nil, err
	}
	data, err := params.HashGet(env, env.MakeSymbol("data"))
	if err != nil {
		return nil, err
	}
	err = h.dht.Put(NewHash(hash.(*zygo.SexpStr).S), []byte(data.(*zygo.SexpStr).S))
	if err != nil {
		put_result = "error: " + err.Error()
	} else {
		put_result = "ok"
	}
	err = result.HashSet(env.MakeSymbol("result"), &zygo.SexpStr{S: put_result})
	return result, err
}

// NewZygoNucleus builds an zygo execution environment with user specified code
func NewZygoNucleus(h *Holochain, code string) (n Nucleus, err error) {
	var z ZygoNucleus
	z.env = zygo.NewGlispSandbox()
	z.env.AddFunction("version",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			return &zygo.SexpStr{S: Version}, nil
		})

	addExtras(&z)

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
					errors.New("2nd argument of expose should be integer")
			}

			err := z.expose(i)
			return zygo.SexpNull, err
		})

	z.env.AddFunction("commit",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			if len(args) != 2 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var entry_type string
			var entry string

			switch t := args[0].(type) {
			case *zygo.SexpStr:
				entry_type = t.S
			default:
				return zygo.SexpNull,
					errors.New("1st argument of commit should be string")
			}

			switch t := args[1].(type) {
			case *zygo.SexpStr:
				entry = t.S
			case *zygo.SexpHash:
				entry = zygo.SexpToJson(t)
			default:
				return zygo.SexpNull,
					errors.New("2nd argument of commit should be string or hash")
			}

			err = h.ValidateEntry(entry_type, entry)
			var headerHash Hash
			if err == nil {
				e := GobEntry{C: entry}
				headerHash, _, err = h.NewEntry(time.Now(), entry_type, &e)

			}
			if err != nil {
				return zygo.SexpNull, err
			}
			var result = zygo.SexpStr{S: headerHash.String()}
			return &result, nil
		})

	z.env.AddFunction("put",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			if len(args) != 1 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var params *zygo.SexpHash
			switch t := args[0].(type) {
			case *zygo.SexpHash:
				params = t
			default:
				return zygo.SexpNull,
					errors.New("argument of put should be hash")
			}
			result, err := z.put(env, h, params)
			return result, err
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

func isPrime(t int64) bool {

	// math.Mod requires floats.
	x := float64(t)

	// 1 or less aren't primes.
	if x <= 1 {
		return false
	}

	// Solve half of the integer set directly
	if math.Mod(x, 2) == 0 {
		return x == 2
	}

	// Main loop. i needs to be float because of math.Mod.
	for i := 3.0; i <= math.Floor(math.Sqrt(x)); i += 2.0 {
		if math.Mod(x, i) == 0 {
			return false
		}
	}

	// It's a prime!
	return true
}

func addExtras(z *ZygoNucleus) {
	z.env.AddFunction("isprime",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {

			switch t := args[0].(type) {
			case *zygo.SexpInt:
				return &zygo.SexpBool{Val: isPrime(t.Val)}, nil
			default:
				return zygo.SexpNull,
					errors.New("argument to isprime should be int")
			}
		})
	z.env.AddFunction("atoi",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {

			var i int64
			var e error
			switch t := args[0].(type) {
			case *zygo.SexpStr:
				i, e = strconv.ParseInt(t.S, 10, 64)
				if e != nil {
					return zygo.SexpNull, e
				}
			default:
				return zygo.SexpNull,
					errors.New("argument to atoi should be string")
			}

			return &zygo.SexpInt{Val: i}, nil
		})
}
