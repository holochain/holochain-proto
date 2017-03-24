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
	peer "github.com/libp2p/go-libp2p-peer"
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
	library    string
}

// Type returns the string value under which this nucleus is registered
func (z *ZygoNucleus) Type() string { return ZygoNucleusType }

// ChainGenesis runs the application genesis function
// this function gets called after the genesis entries are added to the chain
func (z *ZygoNucleus) ChainGenesis() (err error) {
	err = z.env.LoadString(`(genesis)`)
	if err != nil {
		return
	}
	result, err := z.env.Run()
	if err != nil {
		err = fmt.Errorf("Error executing genesis: %v", err)
		return
	}
	switch result.(type) {
	case *zygo.SexpBool:
		r := result.(*zygo.SexpBool).Val
		if !r {
			err = fmt.Errorf("genesis failed")
		}
	case *zygo.SexpSentinel:
		err = errors.New("genesis should return boolean, got nil")

	default:
		err = errors.New("genesis should return boolean, got: " + fmt.Sprintf("%v", result))
	}
	return

}

// ValidateEntry checks the contents of an entry against the validation rules
func (z *ZygoNucleus) ValidateEntry(d *EntryDef, entry Entry, props *ValidationProps) (err error) {
	c := entry.Content().(string)
	// @todo handle JSON if schema type is different
	var e string
	switch d.DataFormat {
	case DataFormatRawZygo:
		e = c
	case DataFormatString:
		e = "\"" + sanitizeString(c) + "\""
	case DataFormatJSON:
		e = fmt.Sprintf(`(unjson (raw "%s"))`, sanitizeString(c))
	default:
		err = errors.New("data format not implemented: " + d.DataFormat)
		return
	}
	// @TODO this is a quick way to build an object from the props structure, but it's
	// expensive, we should just build the Javascript directly and not make the VM parse it
	var b []byte
	b, err = json.Marshal(props)
	if err != nil {
		return
	}
	s := sanitizeString(string(b))
	err = z.env.LoadString(fmt.Sprintf(`(validate "%s" %s (unjson (raw "%s")))`, d.Name, e, s))
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
			err = fmt.Errorf("Invalid entry: %v", entry.Content())
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
	s = strings.Replace(s, "\"", "\\\"", -1)
	return s
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
		if params.(string) == "" {
			code = fmt.Sprintf(`(json (%s (raw "%s")))`, iface, sanitizeString(params.(string)))
		} else {
			code = fmt.Sprintf(`(json (%s (unjson (raw "%s"))))`, iface, sanitizeString(params.(string)))
		}
	default:
		err = errors.New("params type not implemented")
		return
	}
	Debugf("Zygo Call: %s", code)
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
				err = errors.New("expected SexpRaw return type")
			}
		}

	}
	return
}

// These are the zygo implementations of the library functions that must available in
// all Nucleii implementations.
var ZygoLibrary = `(def HC_STRING 0) (def HC_JSON 1) (def HC_Version "` + VersionStr + `")`

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
func (z *ZygoNucleus) putmeta(env *zygo.Glisp, h *Holochain, hash string, metaHash string, metaTag string) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}
	var key Hash
	key, err = NewHash(hash)
	if err != nil {
		return
	}
	var metaKey Hash
	metaKey, err = NewHash(metaHash)
	if err != nil {
		return
	}

	err = h.dht.SendPutMeta(MetaReq{O: key, M: metaKey, T: metaTag})
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

// getmeta exposes GetMeta to zygo
func (z *ZygoNucleus) getmeta(env *zygo.Glisp, h *Holochain, metahash string, metaTag string) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}

	var metakey Hash
	metakey, err = NewHash(metahash)
	if err != nil {
		return
	}

	response, err := h.dht.SendGetMeta(MetaQuery{H: metakey, T: metaTag})
	if err == nil {
		switch t := response.(type) {
		case MetaQueryResp:
			// @TODO figure out encoding by entry type.
			j, err := json.Marshal(t.Entries)
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
			return &zygo.SexpStr{S: VersionStr}, nil
		})

	addExtras(&z)

	// use a closure so that the registered zygo function can call Expose on the correct ZygoNucleus obj

	z.env.AddFunction("debug",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			if len(args) != 1 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var msg string

			switch t := args[0].(type) {
			case *zygo.SexpStr:
				msg = t.S
			default:
				return zygo.SexpNull,
					errors.New("argument of debug should be string")
			}

			h.config.Loggers.App.p(msg)
			return zygo.SexpNull, err
		})

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

			e := GobEntry{C: entry}
			var l int
			var hash Hash
			var header *Header
			l, hash, header, err = h.chain.PrepareHeader(h.hashSpec, time.Now(), entryType, &e, h.agent.PrivKey())
			if err != nil {
				return zygo.SexpNull, err
			}

			p := ValidationProps{
				Sources: []string{peer.IDB58Encode(h.id)},
				Hash:    hash.String(),
			}

			err = h.ValidateEntry(entryType, &e, &p)

			if err == nil {
				err = h.chain.addEntry(l, hash, header, &e)
			}

			if err != nil {
				return zygo.SexpNull, err
			}
			var result = zygo.SexpStr{S: header.EntryLink.String()}
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
			switch t := args[1].(type) {
			case *zygo.SexpStr:
				metahashstr = t.S
			default:
				return zygo.SexpNull,
					errors.New("2nd argument of putmeta should be string")
			}
			var typestr string
			switch t := args[2].(type) {
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
			switch t := args[1].(type) {
			case *zygo.SexpStr:
				typestr = t.S
			default:
				return zygo.SexpNull,
					errors.New("2nd argument of getmeta should be string")
			}
			result, err := z.getmeta(env, h, hashstr, typestr)
			return result, err
		})

	l := ZygoLibrary
	if h != nil {
		l += fmt.Sprintf(`(def App_Name "%s")(def App_DNA_Hash "%s")(def App_Agent_Hash "%s")(def App_Agent_String "%s")(def App_Key_Hash "%s")`, h.Name, h.dnaHash, h.agentHash, h.Agent().Name(), peer.IDB58Encode(h.id))
	}
	z.library = l

	_, err = z.Run(l + code)
	if err != nil {
		return
	}
	n = &z
	return
}

// Run executes zygo code
func (z *ZygoNucleus) Run(code string) (result zygo.Sexp, err error) {
	c := fmt.Sprintf("(begin %s %s)", z.library, code)
	err = z.env.LoadString(c)
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
