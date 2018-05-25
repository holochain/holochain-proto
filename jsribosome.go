// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// JSRibosome implements a javascript use of the Ribosome interface

package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/holochain/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/robertkrimen/otto"
	"strings"
	"time"
)

const (
	JSRibosomeType = "js"

	ErrHandlingReturnErrorsStr = "returnErrorValue"
	ErrHandlingThrowErrorsStr  = "throwErrors"
)

// JSRibosome holds data needed for the Javascript VM
type JSRibosome struct {
	h          *Holochain
	zome       *Zome
	vm         *otto.Otto
	lastResult *otto.Value
}

// Type returns the string value under which this ribosome is registered
func (jsr *JSRibosome) Type() string { return JSRibosomeType }

// ChainGenesis runs the application genesis function
// this function gets called after the genesis entries are added to the chain
func (jsr *JSRibosome) ChainGenesis() (err error) {
	err = jsr.boolFn("genesis", "")
	return
}

// BridgeGenesis runs the bridging genesis function
// this function gets called on both sides of the bridging
func (jsr *JSRibosome) BridgeGenesis(side int, dnaHash Hash, data string) (err error) {
	err = jsr.boolFn("bridgeGenesis", fmt.Sprintf(`%d,"%s","%s"`, side, dnaHash.String(), jsSanitizeString(data)))
	return
}

func (jsr *JSRibosome) boolFn(fnName string, args string) (err error) {
	var v otto.Value
	v, err = jsr.vm.Run(fnName + "(" + args + ")")

	if err != nil {
		err = fmt.Errorf("Error executing %s: %v", fnName, err)
		return
	}
	if v.IsBoolean() {
		var b bool
		b, err = v.ToBoolean()
		if err != nil {
			return
		}
		if !b {
			err = fmt.Errorf("%s failed", fnName)
		}

	} else {
		err = fmt.Errorf("%s should return boolean, got: %v", fnName, v)
	}
	return
}

// Receive calls the app receive function for node-to-node messages
func (jsr *JSRibosome) Receive(from string, msg string) (response string, err error) {
	var code string
	fnName := "receive"

	code = fmt.Sprintf(`JSON.stringify(%s("%s",JSON.parse("%s")))`, fnName, from, jsSanitizeString(msg))
	jsr.h.Debug(code)
	var v otto.Value
	v, err = jsr.vm.Run(code)
	if err != nil {
		err = fmt.Errorf("Error executing %s: %v", fnName, err)
		return
	}
	response, err = v.ToString()
	return
}

// BundleCancel calls the app bundleCanceled function
func (jsr *JSRibosome) BundleCanceled(reason string) (response string, err error) {
	var code string
	fnName := "bundleCanceled"
	bundle := jsr.h.chain.BundleStarted()
	if bundle == nil {
		err = ErrBundleNotStarted
		return
	}

	code = fmt.Sprintf(`%s("%s",JSON.parse("%s"))`, fnName, jsSanitizeString(reason), jsSanitizeString(bundle.userParam))
	jsr.h.Debug(code)
	var v otto.Value
	v, err = jsr.vm.Run(code)
	if err != nil {
		err = fmt.Errorf("Error executing %s: %v", fnName, err)
		return
	}
	response, err = v.ToString()
	return
}

// ValidatePackagingRequest calls the app for a validation packaging request for an action
func (jsr *JSRibosome) ValidatePackagingRequest(action ValidatingAction, def *EntryDef) (req PackagingReq, err error) {
	var code string
	fnName := "validate" + strings.Title(action.Name()) + "Pkg"
	code = fmt.Sprintf(`%s("%s")`, fnName, def.Name)
	jsr.h.Debug(code)
	var v otto.Value
	v, err = jsr.vm.Run(code)
	if err != nil {
		err = fmt.Errorf("Error executing %s: %v", fnName, err)
		return
	}
	if v.IsObject() {
		var m interface{}
		m, err = v.Export()
		if err != nil {
			return
		}
		req = m.(map[string]interface{})
	} else if v.IsNull() {

	} else {
		err = fmt.Errorf("%s should return null or object, got: %v", fnName, v)
	}
	return
}

func prepareJSEntryArgs(def *EntryDef, entry Entry, header *Header) (args string, err error) {
	entryStr := entry.Content().(string)
	switch def.DataFormat {
	case DataFormatRawJS:
		args = entryStr
	case DataFormatString:
		args = "\"" + jsSanitizeString(entryStr) + "\""
	case DataFormatLinks:
		fallthrough
	case DataFormatJSON:
		args = fmt.Sprintf(`JSON.parse("%s")`, jsSanitizeString(entryStr))
	default:
		err = errors.New("data format not implemented: " + def.DataFormat)
		return
	}
	var hdr string
	if header != nil {
		hdr = fmt.Sprintf(
			`{"EntryLink":"%s","Type":"%s","Time":"%s"}`,
			header.EntryLink.String(),
			header.Type,
			header.Time.UTC().Format(time.RFC3339),
		)
	} else {
		hdr = `{"EntryLink":"","Type":"","Time":""}`
	}
	args += "," + hdr
	return
}

func prepareJSValidateArgs(action Action, def *EntryDef) (args string, err error) {
	switch t := action.(type) {
	case *ActionPut:
		args, err = prepareJSEntryArgs(def, t.entry, t.header)
	case *ActionCommit:
		args, err = prepareJSEntryArgs(def, t.entry, t.header)
	case *ActionMod:
		args, err = prepareJSEntryArgs(def, t.entry, t.header)
		if err == nil {
			args += fmt.Sprintf(`,"%s"`, t.replaces.String())
		}
	case *ActionDel:
		args = fmt.Sprintf(`"%s"`, t.entry.Hash.String())
	case *ActionLink:
		var j []byte
		j, err = json.Marshal(t.links)
		if err == nil {
			args = fmt.Sprintf(`"%s",JSON.parse("%s")`, t.validationBase.String(), jsSanitizeString(string(j)))
		}
	default:
		err = fmt.Errorf("can't prepare args for %T: ", t)
		return
	}
	return
}

func buildJSValidateAction(action Action, def *EntryDef, pkg *ValidationPackage, sources []string) (code string, err error) {
	fnName := "validate" + strings.Title(action.Name())
	var args string
	args, err = prepareJSValidateArgs(action, def)
	if err != nil {
		return
	}
	srcs := mkJSSources(sources)

	var pkgObj string
	if pkg == nil || pkg.Chain == nil {
		pkgObj = "{}"
	} else {
		var j []byte
		j, err = json.Marshal(pkg.Chain)
		if err != nil {
			return
		}
		pkgObj = fmt.Sprintf(`{"Chain":%s}`, j)
	}
	code = fmt.Sprintf(`%s("%s",%s,%s,%s)`, fnName, def.Name, args, pkgObj, srcs)

	return
}

// ValidateAction builds the correct validation function based on the action an calls it
func (jsr *JSRibosome) ValidateAction(action Action, def *EntryDef, pkg *ValidationPackage, sources []string) (err error) {
	var code string
	code, err = buildJSValidateAction(action, def, pkg, sources)
	if err != nil {
		return
	}
	jsr.h.Debug(code)
	err = jsr.runValidate(action.Name(), code)
	return
}

func mkJSSources(sources []string) (srcs string) {
	srcs = `["` + strings.Join(sources, `","`) + `"]`
	return
}

func (jsr *JSRibosome) prepareJSValidateEntryArgs(def *EntryDef, entry Entry, sources []string) (e string, srcs string, err error) {
	c := entry.Content().(string)
	switch def.DataFormat {
	case DataFormatRawJS:
		e = c
	case DataFormatString:
		e = "\"" + jsSanitizeString(c) + "\""
	case DataFormatLinks:
		fallthrough
	case DataFormatJSON:
		e = fmt.Sprintf(`JSON.parse("%s")`, jsSanitizeString(c))
	default:
		err = errors.New("data format not implemented: " + def.DataFormat)
		return
	}
	srcs = mkJSSources(sources)
	return
}

func (jsr *JSRibosome) runValidate(fnName string, code string) (err error) {
	var v otto.Value
	v, err = jsr.vm.Run(code)
	if err != nil {
		err = fmt.Errorf("Error executing %s: %v", fnName, err)
		return
	}
	if v.IsBoolean() {
		var b bool
		b, err = v.ToBoolean()
		if err != nil {
			return
		}
		if !b {
			err = ValidationFailed()
		}
	} else if v.IsString() {
		var s string
		s, err = v.ToString()
		if err != nil {
			return
		}
		if s != "" {
			err = ValidationFailed(s)
		}

	} else {
		err = fmt.Errorf("%s should return boolean or string, got: %v", fnName, v)
	}
	return
}

func (jsr *JSRibosome) validateEntry(fnName string, def *EntryDef, entry Entry, header *Header, sources []string) (err error) {

	e, srcs, err := jsr.prepareJSValidateEntryArgs(def, entry, sources)
	if err != nil {
		return
	}

	hdr := fmt.Sprintf(
		`{"EntryLink":"%s","Type":"%s","Time":"%s"}`,
		header.EntryLink.String(),
		header.Type,
		header.Time.UTC().Format(time.RFC3339),
	)

	code := fmt.Sprintf(`%s("%s",%s,%s,%s)`, fnName, def.Name, e, hdr, srcs)
	jsr.h.Debugf("%s: %s", fnName, code)
	err = jsr.runValidate(fnName, code)
	return
}

const (
	JSLibrary = `var HC={Version:` + `"` + VersionStr + "\"" +
		`SysEntryType:{` +
		`DNA:"` + DNAEntryType + `",` +
		`Agent:"` + AgentEntryType + `",` +
		`Key:"` + KeyEntryType + `",` +
		`Headers:"` + HeadersEntryType + `"` +
		`Del:"` + DelEntryType + `"` +
		`}` +
		`HashNotFound:null` +
		`,Status:{Live:` + StatusLiveVal +
		`,Rejected:` + StatusRejectedVal +
		`,Deleted:` + StatusDeletedVal +
		`,Modified:` + StatusModifiedVal +
		`,Any:` + StatusAnyVal +
		"}" +
		`,GetMask:{Default:` + GetMaskDefaultStr +
		`,Entry:` + GetMaskEntryStr +
		`,EntryType:` + GetMaskEntryTypeStr +
		`,Sources:` + GetMaskSourcesStr +
		`,All:` + GetMaskAllStr +
		"}" +
		`,LinkAction:{Add:"` + AddLinkAction + `",Del:"` + DelLinkAction + `"}` +
		`,PkgReq:{Chain:"` + PkgReqChain + `"` +
		`,ChainOpt:{None:` + PkgReqChainOptNoneStr +
		`,Headers:` + PkgReqChainOptHeadersStr +
		`,Entries:` + PkgReqChainOptEntriesStr +
		`,Full:` + PkgReqChainOptFullStr +
		"}" +
		"}" +
		`,Bridge:{Caller:` + BridgeCallerStr +
		`,Callee:` + BridgeCalleeStr +
		"}" +
		`,BundleCancel:{` +
		`Reason:{UserCancel:"` + BundleCancelReasonUserCancel +
		`",Timeout:"` + BundleCancelReasonTimeout +
		`"},Response:{OK:"` + BundleCancelResponseOK +
		`",Commit:"` + BundleCancelResponseCommit +
		`"}}` +
		`Migrate:{Close:"` + MigrateTypeClose + `",Open:"` + MigrateTypeOpen + `"}` +
		`};`
)

// jsSanatizeString makes sure all quotes are quoted and returns are removed
func jsSanitizeString(s string) string {
	s = strings.Replace(s, `\`, "%%%slash%%%", -1)
	s = strings.Replace(s, "\n", "\\n", -1)
	s = strings.Replace(s, "\t", "\\t", -1)
	s = strings.Replace(s, "\r", "", -1)
	s = strings.Replace(s, "\"", "\\\"", -1)
	s = strings.Replace(s, "%%%slash%%%", `\\`, -1)
	return s
}

// Call calls the zygo function that was registered with expose
func (jsr *JSRibosome) Call(fn *FunctionDef, params interface{}) (result interface{}, err error) {
	var code string
	switch fn.CallingType {
	case STRING_CALLING:
		code = fmt.Sprintf(`%s("%s");`, fn.Name, jsSanitizeString(params.(string)))
	case JSON_CALLING:
		if params.(string) == "" {
			code = fmt.Sprintf(`JSON.stringify(%s());`, fn.Name)
		} else {
			p := jsSanitizeString(params.(string))
			code = fmt.Sprintf(`JSON.stringify(%s(JSON.parse("%s")));`, fn.Name, p)
		}
	default:
		err = errors.New("params type not implemented")
		return
	}
	jsr.h.Debugf("JS Call: %s", code)
	var v otto.Value
	v, err = jsr.vm.Run(code)
	if err == nil {
		if v.IsObject() && v.Class() == "Error" {
			jsr.h.Debugf("JS Error:\n%v", v)
			var message otto.Value
			message, err = v.Object().Get("message")
			if err == nil {
				err = errors.New(message.String())
			}
		} else {
			result, err = v.ToString()
		}
	}
	return
}

// jsProcessArgs processes oArgs according to the args spec filling args[].value with the converted value
func jsProcessArgs(jsr *JSRibosome, args []Arg, oArgs []otto.Value) (err error) {
	err = checkArgCount(args, len(oArgs))
	if err != nil {
		return err
	}

	// check arg types
	for i, arg := range oArgs {
		if arg.IsUndefined() && args[i].Optional {
			return
		}
		switch args[i].Type {
		case StringArg:
			if arg.IsString() {
				args[i].value, _ = arg.ToString()
			} else {
				return argErr("string", i+1, args[i])
			}
		case HashArg:
			if arg.IsString() {
				str, _ := arg.ToString()
				var hash Hash
				hash, err = NewHash(str)
				if err != nil {
					return
				}
				args[i].value = hash
			} else {
				return argErr("string", i+1, args[i])
			}
		case IntArg:
			if arg.IsNumber() {
				integer, err := arg.ToInteger()
				if err != nil {
					return err
				}
				args[i].value = integer
			} else {
				return argErr("int", i+1, args[i])
			}
		case BoolArg:
			if arg.IsBoolean() {
				boolean, err := arg.ToBoolean()
				if err != nil {
					return err
				}
				args[i].value = boolean
			} else {
				return argErr("boolean", i+1, args[i])
			}
		case ArgsArg:
			if arg.IsString() {
				str, err := arg.ToString()
				if err != nil {
					return err
				}
				args[i].value = str
			} else if arg.IsObject() {
				v, err := jsr.vm.Call("JSON.stringify", nil, arg)
				if err != nil {
					return err
				}
				entry, err := v.ToString()
				if err != nil {
					return err
				}
				args[i].value = entry

			} else {
				return argErr("string or object", i+1, args[i])
			}
		case EntryArg:
			// this a special case in that all EntryArgs must be preceeded by
			// string arg that specifies the entry type
			entryType, err := oArgs[i-1].ToString()
			if err != nil {
				return err
			}
			_, def, err := jsr.h.GetEntryDef(entryType)
			if err != nil {
				return err
			}
			var entry string
			switch def.DataFormat {
			case DataFormatRawJS:
				fallthrough
			case DataFormatRawZygo:
				fallthrough
			case DataFormatString:
				if !arg.IsString() {
					return argErr("string", i+1, args[i])
				}
				entry, err = arg.ToString()
				if err != nil {
					return err
				}
			case DataFormatLinks:
				if !arg.IsObject() {
					return argErr("object", i+1, args[i])
				}
				fallthrough
			case DataFormatJSON:
				v, err := jsr.vm.Call("JSON.stringify", nil, arg)
				if err != nil {
					return err
				}
				entry, err = v.ToString()
				if err != nil {
					return err
				}
			default:
				err = errors.New("data format not implemented: " + def.DataFormat)
				return err
			}

			args[i].value = entry
		case MapArg:
			if arg.IsObject() {
				m, err := arg.Export()
				if err != nil {
					return err
				}
				args[i].value = m
			} else {
				return argErr("object", i+1, args[i])
			}
		case ToStrArg:
			var str string
			if arg.IsObject() {
				v, err := jsr.vm.Call("JSON.stringify", nil, arg)
				if err != nil {
					return err
				}
				str, err = v.ToString()
				if err != nil {
					return err
				}
			} else {
				str, _ = arg.ToString()
			}
			args[i].value = str
		}
	}
	return
}

const (
	HolochainErrorPrefix = "HolochainError"
)

func mkOttoErr(jsr *JSRibosome, msg string) otto.Value {
	return jsr.vm.MakeCustomError(HolochainErrorPrefix, msg)
}

func numInterfaceToInt(num interface{}) (val int, ok bool) {
	ok = true
	switch t := num.(type) {
	case int64:
		val = int(t)
	case float64:
		val = int(t)
	case int:
		val = t
	default:
		ok = false
	}
	return
}

type fnData struct {
	apiFn APIFunction
	f     func([]Arg, APIFunction, otto.FunctionCall) (otto.Value, error)
}

func makeOttoObjectFromGetResp(h *Holochain, jsr *JSRibosome, getResp *GetResp) (result interface{}, err error) {
	_, def, err := h.GetEntryDef(getResp.EntryType)
	if err != nil {
		return
	}
	if def.DataFormat == DataFormatJSON {
		json := getResp.Entry.Content().(string)
		code := `(` + json + `)`
		result, err = jsr.vm.Object(code)
	} else {
		result = getResp.Entry.Content().(string)
	}
	return
}

// NewJSRibosome factory function to build a javascript execution environment for a zome
func NewJSRibosome(h *Holochain, zome *Zome) (n Ribosome, err error) {
	jsr := JSRibosome{
		h:    h,
		zome: zome,
		vm:   otto.New(),
	}

	funcs := map[string]fnData{
		"property": fnData{
			apiFn: &APIFnProperty{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnProperty)
				f.prop = args[0].value.(string)

				var p interface{}
				p, err = f.Call(h)
				if err != nil {
					return otto.UndefinedValue(), nil
				}
				result, err = jsr.vm.ToValue(p)
				return
			},
		},
		"debug": fnData{
			apiFn: &APIFnDebug{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnDebug)
				f.msg = args[0].value.(string)
				f.Call(h)
				return otto.UndefinedValue(), nil
			},
		},
		"makeHash": fnData{
			apiFn: &APIFnMakeHash{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnMakeHash)
				f.entryType = args[0].value.(string)
				f.entry = &GobEntry{C: args[1].value.(string)}
				var r interface{}
				r, err = f.Call(h)
				if err != nil {
					return
				}
				var entryHash Hash
				if r != nil {
					entryHash = r.(Hash)
				}
				result, _ = jsr.vm.ToValue(entryHash.String())
				return result, nil
			},
		},
		"getBridges": fnData{
			apiFn: &APIFnGetBridges{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnGetBridges)
				var r interface{}
				r, err = f.Call(h)
				if err != nil {
					return
				}
				var code string
				for i, b := range r.([]Bridge) {
					if i > 0 {
						code += ","
					}
					if b.Side == BridgeCallee {
						code += fmt.Sprintf(`{Side:%d,Token:"%s"}`, b.Side, b.Token)
					} else {
						code += fmt.Sprintf(`{Side:%d,CalleeApp:"%s",CalleeName:"%s"}`, b.Side, b.CalleeApp.String(), b.CalleeName)
					}
				}
				code = "[" + code + "]"
				object, _ := jsr.vm.Object(code)
				result, _ = jsr.vm.ToValue(object)
				return
			},
		},
		"sign": fnData{
			apiFn: &APIFnSign{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnSign)
				f.data = []byte(args[0].value.(string))
				var r interface{}
				r, err = f.Call(h)
				if err != nil {
					return
				}
				var b58sig string
				if r != nil {
					b58sig = r.(string)
				}
				result, _ = jsr.vm.ToValue(b58sig)
				return
			},
		},
		"verifySignature": fnData{
			apiFn: &APIFnVerifySignature{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnVerifySignature)
				f.b58signature = args[0].value.(string)
				f.data = args[1].value.(string)
				f.b58pubKey = args[2].value.(string)
				var r interface{}
				r, err = f.Call(h)
				if err != nil {
					return
				}
				result, err = jsr.vm.ToValue(r)
				if err != nil {
					return
				}
				return
			},
		},
		"send": fnData{
			apiFn: &APIFnSend{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnSend)
				a := &f.action
				a.to, err = peer.IDB58Decode(args[0].value.(Hash).String())
				if err != nil {
					return
				}
				msg := args[1].value.(map[string]interface{})
				var j []byte
				j, err = json.Marshal(msg)
				if err != nil {
					return
				}

				a.msg.ZomeType = jsr.zome.Name
				a.msg.Body = string(j)

				if args[2].value != nil {
					a.options = &SendOptions{}
					opts := args[2].value.(map[string]interface{})
					cbmap, ok := opts["Callback"]
					if ok {
						callback := Callback{zomeType: zome.Name}
						v, ok := cbmap.(map[string]interface{})["Function"]
						if !ok {
							err = errors.New("callback option requires Function")
							return
						}
						callback.Function = v.(string)
						v, ok = cbmap.(map[string]interface{})["ID"]
						if !ok {
							err = errors.New("callback option requires ID")
							return
						}
						callback.ID = v.(string)
						a.options.Callback = &callback
					}
					timeout, ok := opts["Timeout"]
					if ok {
						a.options.Timeout = int(timeout.(int64))
					}
				}

				var r interface{}
				r, err = f.Call(h)
				if err != nil {
					return
				}
				result, err = jsr.vm.ToValue(r)
				return
			},
		},
		"call": fnData{
			apiFn: &APIFnCall{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnCall)
				f.zome = args[0].value.(string)
				var zome *Zome
				zome, err = h.GetZome(f.zome)
				if err != nil {
					return
				}
				f.function = args[1].value.(string)
				var fn *FunctionDef
				fn, err = zome.GetFunctionDef(f.function)
				if err != nil {
					return
				}
				if fn.CallingType == JSON_CALLING {
					/* this is a mistake.
					if !call.ArgumentList[2].IsObject() {
								return mkOttoErr(&jsr, "function calling type requires object argument type")
							}*/
				}
				f.args = args[2].value.(string)

				var r interface{}
				r, err = f.Call(h)
				if err != nil {
					return
				}

				result, err = jsr.vm.ToValue(r)
				return
			},
		},
		"bridge": fnData{
			apiFn: &APIFnBridge{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnBridge)
				hash := args[0].value.(Hash)
				f.token, f.url, err = h.GetBridgeToken(hash)
				if err != nil {
					return
				}

				f.zome = args[1].value.(string)
				f.function = args[2].value.(string)
				f.args = args[3].value.(string)

				var r interface{}
				r, err = f.Call(h)
				if err != nil {
					return
				}
				result, err = jsr.vm.ToValue(r)
				return
			},
		},
		"commit": fnData{
			apiFn: &APIFnCommit{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnCommit)
				entryType := args[0].value.(string)
				entryStr := args[1].value.(string)
				var r interface{}
				entry := GobEntry{C: entryStr}
				f.action.entryType = entryType
				f.action.entry = &entry
				r, err = f.Call(h)
				if err != nil {
					return
				}
				var entryHash Hash
				if r != nil {
					entryHash = r.(Hash)
				}

				result, err = jsr.vm.ToValue(entryHash.String())
				return
			},
		},
		"migrate": fnData{
			apiFn: &APIFnMigrate{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnMigrate)
				migrationType := args[0].value.(string)
				Chain := args[1].value.(Hash)
				User := args[2].value.(Hash)
				var r interface{}
				f.action.entry.Type = migrationType
				f.action.entry.Chain = Chain
				f.action.entry.User = User
				r, err = f.Call(h)
				if err != nil {
					return
				}
				var entryHash Hash
				if r != nil {
					entryHash = r.(Hash)
				}

				result, err = jsr.vm.ToValue(entryHash.String())
				return
			},
		},

		"query": fnData{
			apiFn: &APIFnQuery{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnQuery)
				if len(call.ArgumentList) == 1 {
					options := QueryOptions{}
					var j []byte
					j, err = json.Marshal(args[0].value)
					if err != nil {
						return
					}
					jsr.h.Debugf("Query options: %s", string(j))
					err = json.Unmarshal(j, &options)
					if err != nil {
						return
					}
					f.options = &options
				}
				var r interface{}
				r, err = f.Call(h)
				if err != nil {
					return
				}
				qr := r.([]QueryResult)

				defs := make(map[string]*EntryDef)
				var code string
				for i, qresult := range qr {
					if i > 0 {
						code += ","
					}
					var entryCode, hashCode, headerCode string
					var returnCount int
					if f.options.Return.Hashes {
						returnCount += 1
						hashCode = `"` + qresult.Header.EntryLink.String() + `"`
					}
					if f.options.Return.Headers {
						returnCount += 1
						headerCode, err = qresult.Header.ToJSON()
						if err != nil {
							return
						}
					}
					if f.options.Return.Entries {
						returnCount += 1

						var def *EntryDef
						var ok bool
						def, ok = defs[qresult.Header.Type]
						if !ok {
							_, def, err = h.GetEntryDef(qresult.Header.Type)
							if err != nil {
								return
							}
							defs[qresult.Header.Type] = def
						}
						r := qresult.Entry.Content()
						switch def.DataFormat {
						case DataFormatRawJS:
							entryCode = r.(string)
						case DataFormatString:
							entryCode = fmt.Sprintf(`"%s"`, jsSanitizeString(r.(string)))
						case DataFormatLinks:
							fallthrough
						case DataFormatJSON:
							entryCode = fmt.Sprintf(`JSON.parse("%s")`, jsSanitizeString(r.(string)))
						default:
							err = errors.New("data format not implemented: " + def.DataFormat)
							return
						}
					}
					if returnCount == 1 {
						code += entryCode + hashCode + headerCode
					} else {
						var c string
						if entryCode != "" {
							c += "Entry:" + entryCode
						}
						if hashCode != "" {
							if c != "" {
								c += ","
							}
							c += "Hash:" + hashCode
						}
						if headerCode != "" {
							if c != "" {
								c += ","
							}
							c += "Header:" + headerCode
						}
						code += "{" + c + "}"
					}

				}
				code = "[" + code + "]"
				jsr.h.Debugf("Query Code:%s\n", code)
				object, _ := jsr.vm.Object(code)
				result, err = jsr.vm.ToValue(object)
				return
			},
		},
		"get": fnData{
			apiFn: &APIFnGet{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnGet)
				options := GetOptions{StatusMask: StatusDefault}
				if len(call.ArgumentList) == 2 {
					opts, ok := args[1].value.(map[string]interface{})
					if ok {
						mask, ok := opts["StatusMask"]
						if ok {
							// otto returns int64 or float64 depending on whether
							// the mask was returned by constant or addition so
							maskval, ok := numInterfaceToInt(mask)
							if !ok {
								err = errors.New(fmt.Sprintf("expecting int StatusMask attribute, got %T", mask))
								return
							}
							options.StatusMask = int(maskval)
						}
						mask, ok = opts["GetMask"]
						if ok {
							maskval, ok := numInterfaceToInt(mask)
							if !ok {
								err = errors.New(fmt.Sprintf("expecting int GetMask attribute, got %T", mask))
								return
							}
							options.GetMask = int(maskval)
						}
						local, ok := opts["Local"]
						if ok {
							options.Local = local.(bool)
						}
					}
				}
				req := GetReq{H: args[0].value.(Hash), StatusMask: options.StatusMask, GetMask: options.GetMask}
				var r interface{}
				f.action = ActionGet{req: req, options: &options}
				r, err = f.Call(h)
				mask := options.GetMask
				if mask == GetMaskDefault {
					mask = GetMaskEntry
				}
				if err == ErrHashNotFound {
					// if the hash wasn't found this isn't actually an error
					// so return nil which is the same as HC.HashNotFound
					err = nil
					result = otto.NullValue()
				} else if err == nil {
					getResp := r.(GetResp)
					var singleValueReturn bool
					if mask&GetMaskEntry != 0 {
						if GetMaskEntry == mask {
							singleValueReturn = true
							var entry interface{}
							entry, err = makeOttoObjectFromGetResp(h, &jsr, &getResp)
							if err != nil {
								return
							}
							result, err = jsr.vm.ToValue(entry)
						}
					}
					if mask&GetMaskEntryType != 0 {
						if GetMaskEntryType == mask {
							singleValueReturn = true
							result, err = jsr.vm.ToValue(getResp.EntryType)
						}
					}
					if mask&GetMaskSources != 0 {
						if GetMaskSources == mask {
							singleValueReturn = true
							result, err = jsr.vm.ToValue(getResp.Sources)
						}
					}
					if err == nil && !singleValueReturn {
						respObj := make(map[string]interface{})
						if mask&GetMaskEntry != 0 {
							var entry interface{}
							entry, err = makeOttoObjectFromGetResp(h, &jsr, &getResp)
							if err != nil {
								return
							}
							respObj["Entry"] = entry
						}
						if mask&GetMaskEntryType != 0 {
							respObj["EntryType"] = getResp.EntryType
						}
						if mask&GetMaskSources != 0 {
							respObj["Sources"] = getResp.Sources
						}
						result, err = jsr.vm.ToValue(respObj)
					}

				}
				return
			},
		},
		"update": fnData{
			apiFn: &APIFnMod{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnMod)

				entryType := args[0].value.(string)
				entryStr := args[1].value.(string)
				replaces := args[2].value.(Hash)

				entry := GobEntry{C: entryStr}
				f.action = *NewModAction(entryType, &entry, replaces)

				var resp interface{}
				resp, err = f.Call(h)
				if err != nil {
					return
				}
				var entryHash Hash
				if resp != nil {
					entryHash = resp.(Hash)
				}
				result, err = jsr.vm.ToValue(entryHash.String())
				return
			},
		},
		"updateAgent": fnData{
			apiFn: &APIFnModAgent{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnModAgent)
				opts := args[0].value.(map[string]interface{})
				id, idok := opts["Identity"]
				if idok {
					f.Identity = AgentIdentity(id.(string))
				}
				rev, revok := opts["Revocation"]
				if revok {
					f.Revocation = rev.(string)
				}
				var resp interface{}
				resp, err = f.Call(h)
				if err != nil {
					return
				}
				var agentEntryHash Hash
				if resp != nil {
					agentEntryHash = resp.(Hash)
				}
				if revok {
					// TODO there should be a better way to set a variable inside that vm.
					// also worried about the re-entrancy here...
					_, err = jsr.vm.Run(`App.Key.Hash="` + h.nodeIDStr + `"`)
					if err != nil {
						return
					}
				}

				// there's always a new agent entry
				_, err = jsr.vm.Run(`App.Agent.TopHash="` + h.agentTopHash.String() + `"`)
				if err != nil {
					return
				}

				// but not always a new identity to update
				if idok {
					_, err = jsr.vm.Run(`App.Agent.String="` + jsSanitizeString(id.(string)) + `"`)
					if err != nil {
						return
					}
				}

				result, err = jsr.vm.ToValue(agentEntryHash.String())

				return
			},
		},
		"remove": fnData{
			apiFn: &APIFnDel{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				entry := DelEntry{
					Hash:    args[0].value.(Hash),
					Message: args[1].value.(string),
				}
				var resp interface{}
				f := _f.(*APIFnDel)
				f.action = *NewDelAction(entry)
				resp, err = f.Call(h)
				if err == nil {
					var entryHash Hash
					if resp != nil {
						entryHash = resp.(Hash)
					}
					result, err = jsr.vm.ToValue(entryHash.String())
				}

				return
			},
		},
		"getLinks": fnData{
			apiFn: &APIFnGetLinks{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				base := args[0].value.(Hash)
				tag := args[1].value.(string)

				l := len(call.ArgumentList)
				options := GetLinksOptions{Load: false, StatusMask: StatusLive}
				if l == 3 {
					opts, ok := args[2].value.(map[string]interface{})
					if ok {
						load, ok := opts["Load"]
						if ok {
							loadval, ok := load.(bool)
							if !ok {
								err = errors.New(fmt.Sprintf("expecting boolean Load attribute in object, got %T", load))
								return
							}
							options.Load = loadval
						}
						mask, ok := opts["StatusMask"]
						if ok {
							maskval, ok := numInterfaceToInt(mask)
							if !ok {
								err = errors.New(fmt.Sprintf("expecting int StatusMask attribute in object, got %T", mask))
								return
							}
							options.StatusMask = int(maskval)
						}
					}
				}
				var response interface{}
				f := _f.(*APIFnGetLinks)
				f.action = *NewGetLinksAction(&LinkQuery{Base: base, T: tag, StatusMask: options.StatusMask}, &options)
				response, err = f.Call(h)

				if err == nil {
					// we build up our response by creating the javascript object
					// that we want and using otto to create it with vm.
					// TODO: is there a faster way to do this?
					lqr := response.(*LinkQueryResp)
					var js string
					for i, th := range lqr.Links {
						var l string
						l = `Hash:"` + th.H + `"`
						if tag == "" {
							l += `,Tag:"` + jsSanitizeString(th.T) + `"`
						}
						if options.Load {
							l += `,EntryType:"` + jsSanitizeString(th.EntryType) + `"`
							l += `,Source:"` + jsSanitizeString(th.Source) + `"`
							var def *EntryDef
							_, def, err = h.GetEntryDef(th.EntryType)
							if err != nil {
								break
							}
							var entry string
							switch def.DataFormat {
							case DataFormatRawJS:
								entry = th.E
							case DataFormatRawZygo:
								fallthrough
							case DataFormatSysKey:
								// key is a b58 encoded public key so the entry is just the string value
								fallthrough
							case DataFormatString:
								entry = `"` + jsSanitizeString(th.E) + `"`
							case DataFormatLinks:
								fallthrough
							case DataFormatJSON:
								entry = `JSON.parse("` + jsSanitizeString(th.E) + `")`
							default:
								err = errors.New("data format not implemented: " + def.DataFormat)
								return
							}

							l += `,Entry:` + entry
						}
						if i > 0 {
							js += ","
						}
						js += `{` + l + `}`
					}
					if err == nil {
						js = `[` + js + `]`
						var obj *otto.Object
						jsr.h.Debugf("getLinks code:\n%s", js)
						obj, err = jsr.vm.Object(js)
						if err == nil {
							result = obj.Value()
						}
					}
				}
				return
			},
		},
		"bundleStart": fnData{
			apiFn: &APIFnStartBundle{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnStartBundle)
				f.timeout = args[0].value.(int64)
				f.userParam = args[1].value.(string)
				_, err = f.Call(h)
				return
			},
		},
		"bundleClose": fnData{
			apiFn: &APIFnCloseBundle{},
			f: func(args []Arg, _f APIFunction, call otto.FunctionCall) (result otto.Value, err error) {
				f := _f.(*APIFnCloseBundle)
				f.commit = args[0].value.(bool)
				_, err = f.Call(h)
				return
			},
		},
	}

	var fnPrefix string
	var returnErrors bool
	val, ok := zome.Config["ErrorHandling"]
	if ok {
		var errHandling string
		errHandling, ok = val.(string)
		if !ok {
			err = errors.New("Expected ErrorHandling config value to be string")
			return nil, err
		}
		switch errHandling {
		case ErrHandlingThrowErrorsStr:
		case ErrHandlingReturnErrorsStr:
			returnErrors = true
		default:
			err = fmt.Errorf("Expected ErrorHandling config value to be '%s' or '%s', was: '%s'", ErrHandlingThrowErrorsStr, ErrHandlingReturnErrorsStr, errHandling)
			return nil, err
		}

	}
	if !returnErrors {
		fnPrefix = "__"
	}

	for name, data := range funcs {
		wfn := makeJSFN(&jsr, name, data)
		err = jsr.vm.Set(fnPrefix+name, wfn)
		if err != nil {
			return nil, err
		}
	}

	l := JSLibrary
	if h != nil {
		l += fmt.Sprintf(`var App = {Name:"%s",DNA:{Hash:"%s"},Agent:{Hash:"%s",TopHash:"%s",String:"%s"},Key:{Hash:"%s"}};`, h.Name(), h.dnaHash, h.agentHash, h.agentTopHash, jsSanitizeString(string(h.Agent().Identity())), h.nodeIDStr)
	}

	if !returnErrors {
		l += `
		function checkForError(func, rtn) {
		    if (rtn != null && (typeof rtn === 'object') && rtn.name == "` + HolochainErrorPrefix + `") {
		        var errsrc = new getErrorSource(4);
		        throw {
		            name: "` + HolochainErrorPrefix + `",
		            function: func,
		            errorMessage: rtn.message,
		            source: errsrc,
		            toString: function () { return JSON.stringify(this); }
		        }
		    }
		    return rtn;
		}

		function getErrorSource(depth) {
		    try {
		        //Throw an error to generate a stack trace
		        throw new Error();
		    }
		    catch (e) {
		        // get the Xth line of the stack trace
		        var line = e.stack.split('\n')[depth];

		        // pull out the useful data
		        var reg = /at (.*) \(.*:(.*):(.*)\)/g.exec(line);
		        if (reg) {
		            this.functionName = reg[1];
		            this.line = reg[2];
		            this.column = reg[3];
		        }
		    }
		}`

		for name, data := range funcs {
			var args []Arg
			args = data.apiFn.Args()

			var argstr string
			switch len(args) {
			case 1:
				argstr = "a"
			case 2:
				argstr = "a,b"
			case 3:
				argstr = "a,b,c"
			case 4:
				argstr = "a,b,c,d"
			}
			l += fmt.Sprintf(`function %s(%s){return checkForError("%s",__%s(%s))}`, name, argstr, name, name, argstr)
		}
	}

	l += `
// helper function to determine if value returned from holochain function is an error
function isErr(result) {
    return (result != null && (typeof result === 'object') && result.name == "` + HolochainErrorPrefix + `");
}`

	_, err = jsr.Run(l + zome.Code)
	if err != nil {
		return
	}
	n = &jsr
	return
}

func makeJSFN(jsr *JSRibosome, name string, data fnData) func(call otto.FunctionCall) (result otto.Value) {
	return func(call otto.FunctionCall) (result otto.Value) {
		var args []Arg
		args = data.apiFn.Args()

		err := jsProcessArgs(jsr, args, call.ArgumentList)
		if err == nil {
			result, err = data.f(args, data.apiFn, call)

		}
		if err != nil {
			result = mkOttoErr(jsr, err.Error())
		}
		return result
	}
}

// Run executes javascript code
func (jsr *JSRibosome) Run(code string) (result interface{}, err error) {
	v, err := jsr.vm.Run(code)
	if err != nil {
		errStr := err.Error()
		if !strings.HasPrefix(errStr, "{") {
			err = fmt.Errorf("Error executing JavaScript: " + errStr)
		}
		return
	}
	jsr.lastResult = &v
	result = &v
	return
}

func (jsr *JSRibosome) RunAsyncSendResponse(response AppMsg, callback string, callbackID string) (result interface{}, err error) {

	code := fmt.Sprintf(`%s(JSON.parse("%s"),"%s")`, callback, jsSanitizeString(response.Body), jsSanitizeString(callbackID))
	jsr.h.Debugf("Calling %s\n", code)
	result, err = jsr.Run(code)

	return
}
