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

func prepareZyEntryArgs(def *EntryDef, entry Entry, header *Header) (args string, err error) {
	entryStr := entry.Content().(string)
	switch def.DataFormat {
	case DataFormatRawZygo:
		args = entryStr
	case DataFormatString:
		args = "\"" + sanitizeZyString(entryStr) + "\""
	case DataFormatLinks:
		fallthrough
	case DataFormatJSON:
		args = fmt.Sprintf(`(unjson (raw "%s"))`, sanitizeZyString(entryStr))
	default:
		err = errors.New("data format not implemented: " + def.DataFormat)
		return
	}

	var hdr string
	if header != nil {
		hdr = fmt.Sprintf(
			`(hash EntryLink:"%s" Type:"%s" Time:"%s")`,
			header.EntryLink.String(),
			header.Type,
			header.Time.UTC().Format(time.RFC3339),
		)
	} else {
		hdr = `""`
	}

	args += " " + hdr
	return
}

func prepareZyValidateArgs(action Action, def *EntryDef) (args string, err error) {
	switch t := action.(type) {
	case *ActionCommit:
		args, err = prepareZyEntryArgs(def, t.entry, t.header)
	case *ActionPut:
		args, err = prepareZyEntryArgs(def, t.entry, t.header)
	case *ActionDel:
		args = fmt.Sprintf(`"%s"`, t.hash.String())
	case *ActionLink:
		var j []byte
		j, err = json.Marshal(t.links)
		if err == nil {
			args = fmt.Sprintf(`"%s" (unjson (raw "%s"))`, t.validationBase.String(), sanitizeZyString(string(j)))
		}
	default:
		err = fmt.Errorf("can't prepare args for %T: ", t)
		return
	}
	return
}

func buildZyValidateAction(action Action, def *EntryDef, sources []string) (code string, err error) {
	fnName := "validate" + strings.Title(action.Name())
	var args string
	args, err = prepareZyValidateArgs(action, def)
	if err != nil {
		return
	}
	srcs := mkZySources(sources)
	code = fmt.Sprintf(`(%s "%s" %s %s)`, fnName, def.Name, args, srcs)

	return
}

// ValidateAction builds the correct validation function based on the action an calls it
func (z *ZygoNucleus) ValidateAction(action Action, def *EntryDef, sources []string) (err error) {
	var code string
	code, err = buildZyValidateAction(action, def, sources)
	if err != nil {
		return
	}
	Debug(code)
	err = z.runValidate(action.Name(), code)
	return
}

func mkZySources(sources []string) (srcs string) {
	var err error
	var b []byte
	b, err = json.Marshal(sources)
	if err != nil {
		return
	}
	srcs = fmt.Sprintf(`(unjson (raw "%s"))`, sanitizeZyString(string(b)))
	return
}

func (z *ZygoNucleus) prepareValidateArgs(def *EntryDef, entry Entry, sources []string) (e string, srcs string, err error) {
	c := entry.Content().(string)
	// @todo handle JSON if schema type is different
	switch def.DataFormat {
	case DataFormatRawZygo:
		e = c
	case DataFormatString:
		e = "\"" + sanitizeZyString(c) + "\""
	case DataFormatLinks:
		fallthrough
	case DataFormatJSON:
		e = fmt.Sprintf(`(unjson (raw "%s"))`, sanitizeZyString(c))
	default:
		err = errors.New("data format not implemented: " + def.DataFormat)
		return
	}
	srcs = mkZySources(sources)
	return
}

func (z *ZygoNucleus) runValidate(fnName string, code string) (err error) {
	err = z.env.LoadString(code)
	if err != nil {
		return
	}
	result, err := z.env.Run()
	if err != nil {
		err = fmt.Errorf("Error executing %s: %v", fnName, err)
		return
	}
	switch result.(type) {
	case *zygo.SexpBool:
		r := result.(*zygo.SexpBool).Val
		if !r {
			err = ValidationFailedErr
		}
	case *zygo.SexpSentinel:
		err = fmt.Errorf("%s should return boolean, got nil", fnName)

	default:
		err = fmt.Errorf("%s should return boolean, got: %v", fnName, result)
	}
	return
}

func (z *ZygoNucleus) validateEntry(fnName string, def *EntryDef, entry Entry, header *Header, sources []string) (err error) {
	e, srcs, err := z.prepareValidateArgs(def, entry, sources)
	if err != nil {
		return
	}

	var hdr string
	if header != nil {
		hdr = fmt.Sprintf(
			`(hash EntryLink:"%s" Type:"%s" Time:"%s")`,
			header.EntryLink.String(),
			header.Type,
			header.Time.UTC().Format(time.RFC3339),
		)
	} else {
		hdr = `""`
	}

	code := fmt.Sprintf(`(%s "%s" %s %s %s)`, fnName, def.Name, e, hdr, srcs)
	Debugf("%s: %s", fnName, code)

	err = z.runValidate(fnName, code)
	if err != nil && err == ValidationFailedErr {
		err = fmt.Errorf("Invalid entry: %v", entry.Content())
	}
	return
}

// sanatizeZyString makes sure all quotes are quoted
func sanitizeZyString(s string) string {
	s = strings.Replace(s, "\"", "\\\"", -1)
	return s
}

// Call calls the zygo function that was registered with expose
func (z *ZygoNucleus) Call(fn *FunctionDef, params interface{}) (result interface{}, err error) {
	var code string
	switch fn.CallingType {
	case STRING_CALLING:
		code = fmt.Sprintf(`(%s "%s")`, fn.Name, sanitizeZyString(params.(string)))
	case JSON_CALLING:
		if params.(string) == "" {
			code = fmt.Sprintf(`(json (%s (raw "%s")))`, fn.Name, sanitizeZyString(params.(string)))
		} else {
			code = fmt.Sprintf(`(json (%s (unjson (raw "%s"))))`, fn.Name, sanitizeZyString(params.(string)))
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
		switch fn.CallingType {
		case STRING_CALLING:
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
		case JSON_CALLING:
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
var ZygoLibrary = `(def HC_Version "` + VersionStr + `")`

// get exposes DHTGet to zygo
func (z *ZygoNucleus) get(env *zygo.Glisp, h *Holochain, hash Hash) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}

	var entry interface{}

	entry, err = NewGetAction(hash).Do(h)
	if err == nil {
		t := entry.(*GobEntry)
		// @TODO figure out encoding by entry type.
		j, err := json.Marshal(t.C)
		if err == nil {
			err = result.HashSet(env.MakeSymbol("result"), &zygo.SexpStr{S: string(j)})
		}
	} else {
		err = result.HashSet(env.MakeSymbol("error"), &zygo.SexpStr{S: err.Error()})
	}
	return result, err
}

// get exposes DHTGet to zygo
func (z *ZygoNucleus) del(env *zygo.Glisp, h *Holochain, hash Hash) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}

	_, err = NewDelAction(hash).Do(h)
	if err == nil {
		err = result.HashSet(env.MakeSymbol("result"), zygo.SexpNull)

	} else {
		err = result.HashSet(env.MakeSymbol("error"), &zygo.SexpStr{S: err.Error()})
	}
	return result, err
}

func (z *ZygoNucleus) delLink(env *zygo.Glisp, h *Holochain, base Hash, link Hash, tag string) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}

	_, err = NewDelLinkAction(&DelLinkReq{Base: base, Link: link, Tag: tag}).Do(h)
	if err == nil {
		err = result.HashSet(env.MakeSymbol("result"), zygo.SexpNull)

	} else {
		err = result.HashSet(env.MakeSymbol("error"), &zygo.SexpStr{S: err.Error()})
	}
	return
}

// getLink exposes GetLink to zygo
func (z *ZygoNucleus) getLink(env *zygo.Glisp, h *Holochain, base Hash, tag string, options GetLinkOptions) (result *zygo.SexpHash, err error) {
	result, err = zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}

	var r interface{}
	r, err = NewGetLinkAction(&LinkQuery{Base: base, T: tag}, &options).Do(h)
	response := r.(*LinkQueryResp)

	if err == nil {
		var j []byte
		j, err = json.Marshal(response.Links)
		if err == nil {
			err = result.HashSet(env.MakeSymbol("result"), &zygo.SexpStr{S: string(j)})
		}
	} else {
		err = result.HashSet(env.MakeSymbol("error"), &zygo.SexpStr{S: err.Error()})
	}
	return result, err
}

func zyProcessArgs(args []Arg, zyArgs []zygo.Sexp) (err error) {
	err = checkArgCount(args, len(zyArgs))
	if err != nil {
		return err
	}

	// check arg types
	for i, a := range zyArgs {
		switch args[i].Type {
		case StringArg:
			var str string
			switch t := a.(type) {
			case *zygo.SexpStr:
				str = t.S
				args[i].value = str
			default:
				return fmt.Errorf("argument %d of %s should be string", i+1, args[i].Name)
			}
		case HashArg:
			switch t := a.(type) {
			case *zygo.SexpStr:
				var hash Hash
				hash, err = NewHash(t.S)
				if err != nil {
					return
				}
				args[i].value = hash
			default:
				return fmt.Errorf("argument %d of %s should be string", i+1, args[i].Name)
			}
		case IntArg:
			var integer int64
			switch t := a.(type) {
			case *zygo.SexpInt:
				integer = t.Val
				args[i].value = integer
			default:
				return fmt.Errorf("argument %d of %s should be int", i+1, args[i].Name)
			}
		case BoolArg:
			var boolean bool
			switch t := a.(type) {
			case *zygo.SexpBool:
				boolean = t.Val
				args[i].value = boolean
			default:
				return fmt.Errorf("argument %d of %s should be boolean", i+1, args[i].Name)
			}
		case EntryArg:
			switch t := a.(type) {
			case *zygo.SexpStr:
				args[i].value = t.S
			case *zygo.SexpHash:
				args[i].value = zygo.SexpToJson(t)
			default:
				return fmt.Errorf("argument %d of %s should be string or hash", i+1, args[i].Name)
			}

		}
	}

	return
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
			case *zygo.SexpInt:
				msg = fmt.Sprintf("%d", t.Val)
			case *zygo.SexpHash:
				msg = zygo.SexpToJson(t)
			case *zygo.SexpArray:
				msg = zygo.SexpToJson(t)
			default:
				return zygo.SexpNull,
					fmt.Errorf("can't convert argument type %T", t)
			}

			h.config.Loggers.App.p(msg)
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
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			var a Action = &ActionCommit{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			entryType := args[0].value.(string)
			entry := args[1].value.(string)
			var r interface{}
			e := GobEntry{C: entry}
			r, err = NewCommitAction(entryType, &e).Do(h)
			if err != nil {
				return zygo.SexpNull, err
			}
			var entryHash Hash
			if r != nil {
				entryHash = r.(Hash)
			}
			var result = zygo.SexpStr{S: entryHash.String()}
			return &result, nil
		})

	z.env.AddFunction("get",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			var a Action = &ActionGet{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			hash := args[0].value.(Hash)

			result, err := z.get(env, h, hash)
			return result, err
		})

	z.env.AddFunction("del",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			var a Action = &ActionDel{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			hash := args[0].value.(Hash)

			result, err := z.del(env, h, hash)
			return result, err
		})

	z.env.AddFunction("getLink",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			l := len(args)
			if l < 2 || l > 3 {
				return zygo.SexpNull, zygo.WrongNargs
			}

			var basestr string
			switch t := args[0].(type) {
			case *zygo.SexpStr:
				basestr = t.S
			default:
				return zygo.SexpNull,
					errors.New("1st argument of getlink should be string")
			}

			var base Hash
			base, err = NewHash(basestr)
			if err != nil {
				return zygo.SexpNull, err
			}

			var tag string
			switch t := args[1].(type) {
			case *zygo.SexpStr:
				tag = t.S
			default:
				return zygo.SexpNull,
					errors.New("2nd argument of getlink should be string")
			}

			options := GetLinkOptions{Load: false}
			if l == 3 {
				switch t := args[2].(type) {
				case *zygo.SexpHash:
					r, err := t.HashGet(z.env, z.env.MakeSymbol("Load"))
					if err == nil {
						switch t := r.(type) {
						case *zygo.SexpBool:
							options.Load = t.Val
						default:
							return zygo.SexpNull,
								errors.New("Load must be a boolean")
						}
					}
				default:
					return zygo.SexpNull,
						errors.New("3rd argument of getlink should be hash")
				}

			}
			result, err := z.getLink(env, h, base, tag, options)
			return result, err
		})

	z.env.AddFunction("delLink",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			var a Action = &ActionDelLink{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			base := args[0].value.(Hash)
			link := args[1].value.(Hash)
			tag := args[2].value.(string)

			result, err := z.delLink(env, h, base, link, tag)
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
