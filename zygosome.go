// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// ZygoRibosome implements a zygomys use of the Ribosome interface

package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
	zygo "github.com/glycerine/zygomys/repl"
	peer "github.com/libp2p/go-libp2p-peer"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	ZygoRibosomeType = "zygo"
)

// ZygoRibosome holds data needed for the Zygo VM
type ZygoRibosome struct {
	zome       *Zome
	env        *zygo.Glisp
	lastResult zygo.Sexp
	library    string
}

// Type returns the string value under which this ribosome is registered
func (z *ZygoRibosome) Type() string { return ZygoRibosomeType }

// ChainGenesis runs the application genesis function
// this function gets called after the genesis entries are added to the chain
func (z *ZygoRibosome) ChainGenesis() (err error) {
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

// Receive calls the app receive function for node-to-node messages
func (z *ZygoRibosome) Receive(from string, msg string) (response string, err error) {
	var code string
	fnName := "receive"

	code = fmt.Sprintf(`(json (%s "%s" (unjson (raw "%s"))))`, fnName, from, sanitizeZyString(msg))
	Debug(code)
	err = z.env.LoadString(code)
	if err != nil {
		return
	}
	var result interface{}
	result, err = z.env.Run()
	if err == nil {
		switch t := result.(type) {
		case *zygo.SexpStr:
			response = t.S
		case *zygo.SexpInt:
			response = fmt.Sprintf("%d", t.Val)
		case *zygo.SexpRaw:
			response = cleanZygoJson(string(t.Val))
		default:
			result = fmt.Sprintf("%v", result)
		}
	}
	return
}

// ValidatePackagingRequest calls the app for a validation packaging request for an action
func (z *ZygoRibosome) ValidatePackagingRequest(action ValidatingAction, def *EntryDef) (req PackagingReq, err error) {
	var code string
	fnName := "validate" + strings.Title(action.Name()) + "Pkg"
	code = fmt.Sprintf(`(%s "%s")`, fnName, def.Name)
	Debug(code)
	err = z.env.LoadString(code)
	if err != nil {
		return
	}
	result, err := z.env.Run()
	if err != nil {
		err = fmt.Errorf("Error executing %s: %v", fnName, err)
		return
	}
	switch v := result.(type) {
	case *zygo.SexpHash:
		j := cleanZygoJson(zygo.SexpToJson(v))
		m := make(PackagingReq)
		err = json.Unmarshal([]byte(j), &m)
		if err != nil {
			return
		}
		delete(m, "zKeyOrder")
		delete(m, "Atype")
		req = m
	case *zygo.SexpSentinel:
	default:
		err = fmt.Errorf("%s should return nil or hash, got: %v", fnName, v)
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
	case *ActionMod:
		args, err = prepareZyEntryArgs(def, t.entry, t.header)
		if err == nil {
			args += fmt.Sprintf(` "%s"`, t.replaces.String())
		}
	case *ActionDel:
		args = fmt.Sprintf(`"%s"`, t.entry.Hash.String())
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

func buildZyValidateAction(action Action, def *EntryDef, pkg *ValidationPackage, sources []string) (code string, err error) {
	fnName := "validate" + strings.Title(action.Name())
	var args string
	args, err = prepareZyValidateArgs(action, def)
	if err != nil {
		return
	}
	srcs := mkZySources(sources)

	var pkgObj string
	if pkg == nil || pkg.Chain == nil {
		pkgObj = "(hash)"
	} else {
		var j []byte
		j, err = json.Marshal(pkg.Chain)
		if err != nil {
			return
		}
		pkgObj = fmt.Sprintf(`(unjson (raw "%s"))`, sanitizeZyString(string(j)))
	}

	code = fmt.Sprintf(`(%s "%s" %s %s %s)`, fnName, def.Name, args, pkgObj, srcs)

	return
}

// ValidateAction builds the correct validation function based on the action an calls it
func (z *ZygoRibosome) ValidateAction(action Action, def *EntryDef, pkg *ValidationPackage, sources []string) (err error) {
	var code string
	code, err = buildZyValidateAction(action, def, pkg, sources)
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

func (z *ZygoRibosome) prepareValidateArgs(def *EntryDef, entry Entry, sources []string) (e string, srcs string, err error) {
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

func (z *ZygoRibosome) runValidate(fnName string, code string) (err error) {
	err = z.env.LoadString(code)
	if err != nil {
		return
	}
	result, err := z.env.Run()
	if err != nil {
		err = fmt.Errorf("Error executing %s: %v", fnName, err)
		return
	}
	switch v := result.(type) {
	case *zygo.SexpBool:
		r := v.Val
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

func (z *ZygoRibosome) validateEntry(fnName string, def *EntryDef, entry Entry, header *Header, sources []string) (err error) {
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
func (z *ZygoRibosome) Call(fn *FunctionDef, params interface{}) (result interface{}, err error) {
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
				result = cleanZygoJson(string(t.Val))
			default:
				err = errors.New("expected SexpRaw return type")
			}
		}

	}
	return
}

// These are the zygo implementations of the library functions that must available in
// all Ribosome implementations.
const (
	ZygoLibrary = `(def HC_Version "` + VersionStr + `")` +
		`(def HC_Status_Live ` + StatusLiveVal + ")" +
		`(def HC_Status_Rejected ` + StatusRejectedVal + ")" +
		`(def HC_Status_Deleted ` + StatusDeletedVal + ")" +
		`(def HC_Status_Modified ` + StatusModifiedVal + ")" +
		`(def HC_Status_Any ` + StatusAnyVal + ")" +
		`(def HC_GetMask_Default ` + GetMaskDefaultStr + ")" +
		`(def HC_GetMask_Entry ` + GetMaskEntryStr + ")" +
		`(def HC_GetMask_EntryType ` + GetMaskEntryTypeStr + ")" +
		`(def HC_GetMask_Sources ` + GetMaskSourcesStr + ")" +
		`(def HC_GetMask_All ` + GetMaskAllStr + ")" +

		`(def HC_LinkAction_Add "` + AddAction + "\")" +
		`(def HC_LinkAction_Del "` + DelAction + "\")" +
		`(def HC_PkgReq_Chain "` + PkgReqChain + "\")" +
		`(def HC_PkgReq_ChainOpt_None "` + PkgReqChainOptNoneStr + "\")" +
		`(def HC_PkgReq_ChainOpt_Headers "` + PkgReqChainOptHeadersStr + "\")" +
		`(def HC_PkgReq_ChainOpt_Entries "` + PkgReqChainOptEntriesStr + "\")" +
		`(def HC_PkgReq_ChainOpt_Full "` + PkgReqChainOptFullStr + "\")"
)

func makeResult(env *zygo.Glisp, resultValue zygo.Sexp, resultError error) (zygo.Sexp, error) {
	result, err := zygo.MakeHash(nil, "hash", env)
	if err != nil {
		return nil, err
	}
	if resultError != nil {
		err = result.HashSet(env.MakeSymbol("error"), &zygo.SexpStr{S: resultError.Error()})
	} else {
		err = result.HashSet(env.MakeSymbol("result"), resultValue)
	}
	return result, err
}

// cleanZygoJson removes zygos crazy crap
func cleanZygoJson(s string) string {
	s = strings.Replace(s, `"Atype":"hash", `, "", -1)
	re := regexp.MustCompile(`, "zKeyOrder":\[[^\]]+\]`)
	s = string(re.ReplaceAll([]byte(s), []byte("")))
	return s
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
				return argErr("string", i+1, args[i])
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
				return argErr("string", i+1, args[i])
			}
		case IntArg:
			var integer int64
			switch t := a.(type) {
			case *zygo.SexpInt:
				integer = t.Val
				args[i].value = integer
			default:
				return argErr("int", i+1, args[i])
			}
		case BoolArg:
			var boolean bool
			switch t := a.(type) {
			case *zygo.SexpBool:
				boolean = t.Val
				args[i].value = boolean
			default:
				return argErr("boolean", i+1, args[i])
			}
		case ArgsArg:
			switch t := a.(type) {
			case *zygo.SexpStr:
				args[i].value = t.S
			case *zygo.SexpHash:
				args[i].value = cleanZygoJson(zygo.SexpToJson(t))
			default:
				return argErr("string or hash", i+1, args[i])
			}
		case EntryArg:
			switch t := a.(type) {
			case *zygo.SexpStr:
				args[i].value = t.S
			case *zygo.SexpHash:
				args[i].value = cleanZygoJson(zygo.SexpToJson(t))
			default:
				return argErr("string or hash", i+1, args[i])
			}
		case MapArg:
			switch t := a.(type) {
			case *zygo.SexpHash:
				j := cleanZygoJson(zygo.SexpToJson(t))
				m := make(map[string]interface{})
				var err = json.Unmarshal([]byte(j), &m)
				if err != nil {
					return err
				}
				args[i].value = m
			default:
				return argErr("hash", i+1, args[i])
			}
		case ToStrArg:
			var str string

			switch t := a.(type) {
			case *zygo.SexpStr:
				str = t.S
			case *zygo.SexpInt:
				str = fmt.Sprintf("%d", t.Val)
			case *zygo.SexpBool:
				if t.Val {
					str = "true"
				} else {
					str = "false"
				}
			case *zygo.SexpHash:
				str = cleanZygoJson(zygo.SexpToJson(t))
			case *zygo.SexpArray:
				str = cleanZygoJson(zygo.SexpToJson(t))
			default:
				return argErr("int, boolean, string, array or hash", i+1, args[i])
			}
			args[i].value = str
		}
	}

	return
}

// NewZygoRibosome factory function to build a zygo execution environment for a zome
func NewZygoRibosome(h *Holochain, zome *Zome) (n Ribosome, err error) {
	z := ZygoRibosome{
		zome: zome,
		env:  zygo.NewGlispSandbox(),
	}

	z.env.AddFunction("version",
		func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
			return &zygo.SexpStr{S: VersionStr}, nil
		})

	addExtras(&z)

	// use a closure so that the registered zygo function can call Expose on the correct ZygoRibosome obj

	z.env.AddFunction("property",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			a := &ActionProperty{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}

			a.prop = args[0].value.(string)

			var p interface{}
			p, err = a.Do(h)

			if err != nil {
				return zygo.SexpNull, err
			}
			result := zygo.SexpStr{S: p.(string)}
			return &result, err
		})

	z.env.AddFunction("debug",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			a := &ActionDebug{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			a.msg = args[0].value.(string)
			a.Do(h)
			return zygo.SexpNull, err
		})

	z.env.AddFunction("makeHash",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			a := &ActionMakeHash{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			a.entry = &GobEntry{C: args[0].value.(string)}
			var r interface{}
			r, err = a.Do(h)
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

	z.env.AddFunction("send",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			a := &ActionSend{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}

			a.to, err = peer.IDB58Decode(args[0].value.(Hash).String())
			if err != nil {
				return zygo.SexpNull, err
			}

			msg := args[1].value.(map[string]interface{})
			var j []byte
			j, err = json.Marshal(msg)
			if err != nil {
				return zygo.SexpNull, err
			}

			a.msg.ZomeType = z.zome.Name
			a.msg.Body = string(j)

			var r interface{}
			r, err = a.Do(h)
			var resp zygo.Sexp
			if err == nil {
				resp = &zygo.SexpStr{S: r.(string)}
			}
			return makeResult(env, resp, err)
		})

	z.env.AddFunction("call",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			a := &ActionCall{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			a.zome = args[0].value.(string)
			var zome *Zome
			zome, err = h.GetZome(a.zome)
			if err != nil {
				return zygo.SexpNull, err
			}
			a.function = args[1].value.(string)
			var fn *FunctionDef
			fn, err = zome.GetFunctionDef(a.function)
			if err != nil {
				return zygo.SexpNull, err
			}
			if fn.CallingType == JSON_CALLING {
				switch zyargs[2].(type) {
				case *zygo.SexpHash:
					a.args = args[2].value
				default:
					return zygo.SexpNull, errors.New("function calling type requires hash argument type")
				}

			} else {
				a.args = args[2].value.(string)
			}
			var r interface{}
			r, err = a.Do(h)
			if err != nil {
				return zygo.SexpNull, err
			}
			return &zygo.SexpStr{S: r.(string)}, err
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
			options := GetOptions{StatusMask: StatusDefault, GetMask: GetMaskDefault}
			if len(zyargs) == 2 {
				opts := args[1].value.(map[string]interface{})
				mask, ok := opts["StatusMask"]
				if ok {
					maskval, ok := mask.(float64)
					if !ok {
						return zygo.SexpNull,
							fmt.Errorf("expecting int StatusMask attribute, got %T", mask)
					}
					options.StatusMask = int(maskval)
				}
				mask, ok = opts["GetMask"]
				if ok {
					maskval, ok := mask.(float64)
					if !ok {
						return zygo.SexpNull,
							fmt.Errorf("expecting int GetMask attribute, got %T", mask)
					}
					options.GetMask = int(maskval)
				}
				local, ok := opts["Local"]
				if ok {
					options.Local = local.(bool)
				}

			}
			req := GetReq{H: args[0].value.(Hash), StatusMask: options.StatusMask, GetMask: options.GetMask}

			var r interface{}
			r, err = NewGetAction(req, &options).Do(h)
			mask := options.GetMask
			if mask == GetMaskDefault {
				mask = GetMaskEntry
			}
			var resultValue zygo.Sexp
			resultValue = zygo.SexpNull
			if err == nil {
				getResp := r.(GetResp)
				var entryStr string
				var singleValueReturn bool
				if mask&GetMaskEntry != 0 {
					j, err := json.Marshal(getResp.Entry.Content())
					if err == nil {
						if GetMaskEntry == mask {
							singleValueReturn = true
							resultValue = &zygo.SexpStr{S: string(j)}
						} else {
							entryStr = string(j)
						}
					}
				}
				if mask&GetMaskEntryType != 0 {
					if GetMaskEntryType == mask {
						singleValueReturn = true
						resultValue = &zygo.SexpStr{S: getResp.EntryType}
					}
				}
				var zSources *zygo.SexpArray
				if mask&GetMaskSources != 0 {
					sources := make([]zygo.Sexp, len(getResp.Sources))
					for i := range getResp.Sources {
						sources[i] = &zygo.SexpStr{S: getResp.Sources[i]}
					}
					zSources = env.NewSexpArray(sources)
					if GetMaskSources == mask {
						singleValueReturn = true
						resultValue = zSources
					}
				}
				if err == nil && !singleValueReturn {
					// build the return object
					var respObj *zygo.SexpHash
					respObj, err = zygo.MakeHash(nil, "hash", env)
					if err == nil {
						resultValue = respObj
						if mask&GetMaskEntry != 0 {
							err = respObj.HashSet(env.MakeSymbol("Entry"), &zygo.SexpStr{S: entryStr})
						}
						if mask&GetMaskEntryType != 0 {
							err = respObj.HashSet(env.MakeSymbol("EntryType"), &zygo.SexpStr{S: getResp.EntryType})
						}
						if mask&GetMaskSources != 0 {
							err = respObj.HashSet(env.MakeSymbol("Sources"), zSources)
						}
					}
				}
			}
			return makeResult(env, resultValue, err)
			//			return result, err
		})

	z.env.AddFunction("update",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			var a Action = &ActionMod{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			entryType := args[0].value.(string)
			entryStr := args[1].value.(string)
			replaces := args[2].value.(Hash)

			entry := GobEntry{C: entryStr}
			resp, err := NewModAction(entryType, &entry, replaces).Do(h)
			if err != nil {
				return zygo.SexpNull, err
			}
			var entryHash Hash
			if resp != nil {
				entryHash = resp.(Hash)
			}
			var result = zygo.SexpStr{S: entryHash.String()}
			return &result, nil
		})

	z.env.AddFunction("remove",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			var a Action = &ActionDel{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			entry := DelEntry{
				Hash:    args[0].value.(Hash),
				Message: args[1].value.(string),
			}
			header, err := h.chain.GetEntryHeader(entry.Hash)
			if err == nil {
				resp, err := NewDelAction(header.Type, entry).Do(h)
				if err != nil {
					return zygo.SexpNull, err
				}
				var entryHash Hash
				if resp != nil {
					entryHash = resp.(Hash)
				}
				return &zygo.SexpStr{S: entryHash.String()}, err
			}
			return zygo.SexpNull, err
		})

	z.env.AddFunction("getLink",
		func(env *zygo.Glisp, name string, zyargs []zygo.Sexp) (zygo.Sexp, error) {
			var a Action = &ActionGetLink{}
			args := a.Args()
			err := zyProcessArgs(args, zyargs)
			if err != nil {
				return zygo.SexpNull, err
			}
			base := args[0].value.(Hash)
			tag := args[1].value.(string)

			options := GetLinkOptions{Load: false, StatusMask: StatusLive}
			if len(zyargs) == 3 {
				opts := args[2].value.(map[string]interface{})
				load, ok := opts["Load"]
				if ok {
					loadval, ok := load.(bool)
					if !ok {
						return zygo.SexpNull,
							fmt.Errorf("expecting boolean Load attribute in object, got %T", load)
					}
					options.Load = loadval
				}
				mask, ok := opts["StatusMask"]
				if ok {
					maskval, ok := mask.(float64)
					if !ok {
						return zygo.SexpNull,
							fmt.Errorf("expecting int StatusMask attribute in object, got %T", mask)
					}
					options.StatusMask = int(maskval)
				}
			}

			var r interface{}
			r, err = NewGetLinkAction(&LinkQuery{Base: base, T: tag, StatusMask: options.StatusMask}, &options).Do(h)
			var resultValue zygo.Sexp
			if err == nil {
				response := r.(*LinkQueryResp)
				resultValue = zygo.SexpNull
				var j []byte
				j, err = json.Marshal(response.Links)
				if err == nil {
					resultValue = &zygo.SexpStr{S: string(j)}
				}
			}
			return makeResult(env, resultValue, err)
		})

	l := ZygoLibrary
	if h != nil {
		l += fmt.Sprintf(`(def App_Name "%s")(def App_DNA_Hash "%s")(def App_Agent_Hash "%s")(def App_Agent_String "%s")(def App_Key_Hash "%s")`, h.nucleus.dna.Name, h.dnaHash, h.agentHash, h.Agent().Identity(), h.nodeIDStr)
	}
	z.library = l

	_, err = z.Run(l + zome.Code)
	if err != nil {
		return
	}
	n = &z
	return
}

// Run executes zygo code
func (z *ZygoRibosome) Run(code string) (result interface{}, err error) {
	c := fmt.Sprintf("(begin %s %s)", z.library, code)
	err = z.env.LoadString(c)
	if err != nil {
		err = errors.New("Zygomys load error: " + err.Error())
		return
	}
	var sexp zygo.Sexp
	sexp, err = z.env.Run()
	if err != nil {
		err = errors.New("Zygomys exec error: " + err.Error())
		return
	}
	z.lastResult = sexp
	result = sexp
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

func addExtras(z *ZygoRibosome) {
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
