// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// ZygoNucleus implements a zygomys use of the Nucleus interface

package holochain

import (
	"encoding/json"
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

// InitChain runs the application init function
// this function gets called after the genesis entries are added to the chain
func (z *ZygoNucleus) InitChain() (err error) {
	err = z.env.LoadString(`(init)`)
	if err != nil {
		return
	}
	result, err := z.env.Run()
	if err != nil {
		err = fmt.Errorf("Error executing init: %v", err)
		return
	}
	switch result.(type) {
	case *zygo.SexpBool:
		r := result.(*zygo.SexpBool).Val
		if !r {
			err = fmt.Errorf("init failed")
		}
	case *zygo.SexpSentinel:
		err = errors.New("init should return boolean, got nil")

	default:
		err = errors.New("init should return boolean, got: " + fmt.Sprintf("%v", result))
	}
	return

}

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
		err = fmt.Errorf("Error executing validate: %v", err)
		return
	}
	switch result.(type) {
	case *zygo.SexpBool:
		r := result.(*zygo.SexpBool).Val
		if !r {
			err = fmt.Errorf("Invalid entry: %v", entry)
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

// Interfaces returns the list of application exposed functions the nucleus
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

// put exposes DHTPut to zygo
func (z *ZygoNucleus) put(env *zygo.Glisp, h *Holochain, hash string) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}
	var key Hash
	key, err = NewHash(hash)
	if err != nil {
		return
	}
	err = h.dht.SendPut(key)
	if err != nil {
		err = result.HashSet(env.MakeSymbol("error"), &zygo.SexpStr{S: err.Error()})
	} else {
		err = result.HashSet(env.MakeSymbol("result"), &zygo.SexpStr{S: "ok"})
	}
	return result, err
}

// putmeta exposes DHTPutMeta to zygo
func (z *ZygoNucleus) putmeta(env *zygo.Glisp, h *Holochain, hash string, metahash string, metatype string) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}
	var key Hash
	key, err = NewHash(hash)
	if err != nil {
		return
	}
	var metakey Hash
	metakey, err = NewHash(metahash)
	if err != nil {
		return
	}

	err = h.dht.SendPutMeta(MetaReq{O: key, M: metakey, T: metatype})
	if err != nil {
		err = result.HashSet(env.MakeSymbol("error"), &zygo.SexpStr{S: err.Error()})
	} else {
		err = result.HashSet(env.MakeSymbol("result"), &zygo.SexpStr{S: "ok"})
	}
	return result, err
}

// get exposes DHTGet to zygo
func (z *ZygoNucleus) get(env *zygo.Glisp, h *Holochain, hash string) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}

	var key Hash
	key, err = NewHash(hash)
	if err != nil {
		return
	}
	response, err := h.dht.SendGet(key)
	if err == nil {
		switch t := response.(type) {
		case *GobEntry:
			// @TODO figure out encoding by entry type.
			j, err := json.Marshal(t.C)
			if err == nil {
				err = result.HashSet(env.MakeSymbol("result"), &zygo.SexpStr{S: string(j)})
			}
		// @TODO what about if the hash was of a header??
		default:
			err = fmt.Errorf("unexpected response type from SendGet: %v", t)
		}
	} else {
		err = result.HashSet(env.MakeSymbol("error"), &zygo.SexpStr{S: err.Error()})
	}
	return result, err
}

// getmeta exposes GetPutMeta to zygo
func (z *ZygoNucleus) getmeta(env *zygo.Glisp, h *Holochain, metahash string, metatype string) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}

	var metakey Hash
	metakey, err = NewHash(metahash)
	if err != nil {
		return
	}

	response, err := h.dht.SendGetMeta(MetaQuery{H: metakey, T: metatype})
	if err == nil {
		switch t := response.(type) {
		case []Entry:
			// @TODO figure out encoding by entry type.
			j, err := json.Marshal(t)
			if err == nil {
				err = result.HashSet(env.MakeSymbol("result"), &zygo.SexpStr{S: string(j)})
			}
		default:
			err = fmt.Errorf("unexpected response type from SendGetMeta: %v", t)
		}
	} else {
		err = result.HashSet(env.MakeSymbol("error"), &zygo.SexpStr{S: err.Error()})
	}
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

	z.env.AddFunction("property",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			if len(args) != 1 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var prop string

			switch t := args[0].(type) {
			case *zygo.SexpStr:
				prop = t.S
			default:
				return zygo.SexpNull,
					errors.New("1st argument of expose should be string")
			}

			p, err := h.GetProperty(prop)
			if err != nil {
				return zygo.SexpNull, err
			}
			result := zygo.SexpStr{S: p}
			return &result, err
		})

	z.env.AddFunction("commit",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			if len(args) != 2 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var entryType string
			var entry string

			switch t := args[0].(type) {
			case *zygo.SexpStr:
				entryType = t.S
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

			err = h.ValidateEntry(entryType, entry)
			var headerHash Hash
			if err == nil {
				e := GobEntry{C: entry}
				headerHash, _, err = h.NewEntry(time.Now(), entryType, &e)

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

			var hashstr string
			switch t := args[0].(type) {
			case *zygo.SexpStr:
				hashstr = t.S
			default:
				return zygo.SexpNull,
					errors.New("argument of put should be string")
			}
			result, err := z.put(env, h, hashstr)
			return result, err
		})

	z.env.AddFunction("get",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			if len(args) != 1 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var hashstr string
			switch t := args[0].(type) {
			case *zygo.SexpStr:
				hashstr = t.S
			default:
				return zygo.SexpNull,
					errors.New("argument of put should be string")
			}
			result, err := z.get(env, h, hashstr)
			return result, err
		})

	z.env.AddFunction("putmeta",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			if len(args) != 3 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var hashstr string
			switch t := args[0].(type) {
			case *zygo.SexpStr:
				hashstr = t.S
			default:
				return zygo.SexpNull,
					errors.New("1st argument of putmeta should be string")
			}
			var metahashstr string
			switch t := args[0].(type) {
			case *zygo.SexpStr:
				metahashstr = t.S
			default:
				return zygo.SexpNull,
					errors.New("2nd argument of putmeta should be string")
			}
			var typestr string
			switch t := args[0].(type) {
			case *zygo.SexpStr:
				typestr = t.S
			default:
				return zygo.SexpNull,
					errors.New("3rd argument of putmeta should be string")
			}
			result, err := z.putmeta(env, h, hashstr, metahashstr, typestr)
			return result, err
		})

	z.env.AddFunction("getmeta",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			if len(args) != 2 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var hashstr string
			switch t := args[0].(type) {
			case *zygo.SexpStr:
				hashstr = t.S
			default:
				return zygo.SexpNull,
					errors.New("1st argument of gettmeta should be string")
			}

			var typestr string
			switch t := args[0].(type) {
			case *zygo.SexpStr:
				typestr = t.S
			default:
				return zygo.SexpNull,
					errors.New("2nd argument of getmeta should be string")
			}
			result, err := z.getmeta(env, h, hashstr, typestr)
			return result, err
		})
	_, err = z.Run(ZygoLibrary + code)
	if err != nil {
		return
	}
	n = &z
	return
}

// Run executes zygo code
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

// extra functions we want to have available for app developers in zygo

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
