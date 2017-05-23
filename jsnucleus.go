// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// JSNucleus implements a javascript use of the Nucleus interface

package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/robertkrimen/otto"
	"strings"
	"time"
)

const (
	JSNucleusType = "js"
)

// JSNucleus holds data needed for the Javascript VM
type JSNucleus struct {
	vm         *otto.Otto
	lastResult *otto.Value
}

// Type returns the string value under which this nucleus is registered
func (z *JSNucleus) Type() string { return JSNucleusType }

// ChainGenesis runs the application genesis function
// this function gets called after the genesis entries are added to the chain
func (z *JSNucleus) ChainGenesis() (err error) {
	v, err := z.vm.Run(`genesis()`)
	if err != nil {
		err = fmt.Errorf("Error executing genesis: %v", err)
		return
	}
	if v.IsBoolean() {
		var b bool
		b, err = v.ToBoolean()
		if err != nil {
			return
		}
		if !b {
			err = fmt.Errorf("genesis failed")
		}

	} else {
		err = fmt.Errorf("genesis should return boolean, got: %v", v)
	}
	return
}

// ValidatePackagingRequest calls the app for a validation packaging request for an action
func (z *JSNucleus) ValidatePackagingRequest(action ValidatingAction, def *EntryDef) (req PackagingReq, err error) {
	var code string
	fnName := "validate" + strings.Title(action.Name()) + "Pkg"
	code = fmt.Sprintf(`%s("%s")`, fnName, def.Name)
	Debug(code)
	var v otto.Value
	v, err = z.vm.Run(code)
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
func (z *JSNucleus) ValidateAction(action Action, def *EntryDef, pkg *ValidationPackage, sources []string) (err error) {
	var code string
	code, err = buildJSValidateAction(action, def, pkg, sources)
	if err != nil {
		return
	}
	Debug(code)
	err = z.runValidate(action.Name(), code)
	return
}

func mkJSSources(sources []string) (srcs string) {
	srcs = `["` + strings.Join(sources, `","`) + `"]`
	return
}

func (z *JSNucleus) prepareJSValidateEntryArgs(def *EntryDef, entry Entry, sources []string) (e string, srcs string, err error) {
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

func (z *JSNucleus) runValidate(fnName string, code string) (err error) {
	var v otto.Value
	v, err = z.vm.Run(code)
	if err != nil {
		err = fmt.Errorf("Error executing %s: %v", fnName, err)
		return
	}
	if v.IsBoolean() {
		if v.IsBoolean() {
			var b bool
			b, err = v.ToBoolean()
			if err != nil {
				return
			}
			if !b {
				err = ValidationFailedErr
			}
		}
	} else {
		err = fmt.Errorf("%s should return boolean, got: %v", fnName, v)
	}
	return
}

func (z *JSNucleus) validateEntry(fnName string, def *EntryDef, entry Entry, header *Header, sources []string) (err error) {

	e, srcs, err := z.prepareJSValidateEntryArgs(def, entry, sources)
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
	Debugf("%s: %s", fnName, code)
	err = z.runValidate(fnName, code)
	if err != nil && err == ValidationFailedErr {
		err = fmt.Errorf("Invalid entry: %v", entry.Content())
	}

	return
}

const (
	JSLibrary = `var HC={Version:` + `"` + VersionStr + "\"" +
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
		`,LinkAction:{Add:"` + AddAction + `",Del:"` + DelAction + `"}` +
		`,PkgReq:{Chain:"` + PkgReqChain + `"` +
		`,ChainOpt:{None:` + PkgReqChainOptNoneStr +
		`,Headers:` + PkgReqChainOptHeadersStr +
		`,Entries:` + PkgReqChainOptEntriesStr +
		`,Full:` + PkgReqChainOptFullStr +
		"}" +
		"}" +
		`};`
)

// jsSanatizeString makes sure all quotes are quoted and returns are removed
func jsSanitizeString(s string) string {
	s0 := strings.Replace(s, "\n", "", -1)
	s1 := strings.Replace(s0, "\r", "", -1)
	s2 := strings.Replace(s1, "\"", "\\\"", -1)
	return s2
}

// Call calls the zygo function that was registered with expose
func (z *JSNucleus) Call(fn *FunctionDef, params interface{}) (result interface{}, err error) {
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
	Debugf("JS Call: %s", code)
	var v otto.Value
	v, err = z.vm.Run(code)
	if err == nil {
		if v.IsObject() && v.Class() == "Error" {
			Debugf("JS Error:\n%v", v)
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
func jsProcessArgs(z *JSNucleus, args []Arg, oArgs []otto.Value) (err error) {
	err = checkArgCount(args, len(oArgs))
	if err != nil {
		return err
	}

	// check arg types
	for i, arg := range oArgs {
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
				v, err := z.vm.Call("JSON.stringify", nil, arg)
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
			if arg.IsString() {
				str, err := arg.ToString()
				if err != nil {
					return err
				}
				args[i].value = str
			} else if arg.IsObject() {
				v, err := z.vm.Call("JSON.stringify", nil, arg)
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
				v, err := z.vm.Call("JSON.stringify", nil, arg)
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

func mkOttoErr(z *JSNucleus, msg string) otto.Value {
	return z.vm.MakeCustomError("HolochainError", msg)
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

// NewJSNucleus builds a javascript execution environment with user specified code
func NewJSNucleus(h *Holochain, code string) (n Nucleus, err error) {
	var z JSNucleus
	z.vm = otto.New()

	err = z.vm.Set("property", func(call otto.FunctionCall) otto.Value {
		a := &ActionProperty{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}

		a.prop = args[0].value.(string)

		var p interface{}
		p, err = a.Do(h)
		if err != nil {
			return otto.UndefinedValue()
		}
		result, _ := z.vm.ToValue(p)
		return result
	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("debug", func(call otto.FunctionCall) otto.Value {
		a := &ActionDebug{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		a.msg = args[0].value.(string)
		a.Do(h)
		return otto.UndefinedValue()
	})

	err = z.vm.Set("makeHash", func(call otto.FunctionCall) otto.Value {
		a := &ActionMakeHash{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}

		a.entry = &GobEntry{C: args[0].value.(string)}
		var r interface{}
		r, err = a.Do(h)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		var entryHash Hash
		if r != nil {
			entryHash = r.(Hash)
		}
		result, _ := z.vm.ToValue(entryHash.String())
		return result
	})

	err = z.vm.Set("call", func(call otto.FunctionCall) otto.Value {
		a := &ActionCall{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		a.zome = args[0].value.(string)
		var zome *Zome
		zome, err = h.GetZome(a.zome)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		a.function = args[1].value.(string)
		var fn *FunctionDef
		fn, err = h.GetFunctionDef(zome, a.function)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		if fn.CallingType == JSON_CALLING {
			if !call.ArgumentList[2].IsObject() {
				return mkOttoErr(&z, "function calling type requires object argument type")
			}
		}
		a.args = args[2].value.(string)

		var r interface{}
		r, err = a.Do(h)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		var result otto.Value
		result, err = z.vm.ToValue(r)

		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		return result
	})

	err = z.vm.Set("commit", func(call otto.FunctionCall) otto.Value {
		var a Action = &ActionCommit{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}

		entryType := args[0].value.(string)
		entryStr := args[1].value.(string)
		var r interface{}
		entry := GobEntry{C: entryStr}
		r, err = NewCommitAction(entryType, &entry).Do(h)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		var entryHash Hash
		if r != nil {
			entryHash = r.(Hash)
		}

		result, _ := z.vm.ToValue(entryHash.String())
		return result
	})
	if err != nil {
		return nil, err
	}
	err = z.vm.Set("get", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionGet{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}

		options := GetOptions{StatusMask: StatusDefault}
		if len(call.ArgumentList) == 2 {
			opts := args[1].value.(map[string]interface{})
			mask, ok := opts["StatusMask"]
			if ok {
				// otto returns int64 or float64 depending on whether
				// the mask was returned by constant or addition so
				maskval, ok := numInterfaceToInt(mask)
				if !ok {
					return mkOttoErr(&z, fmt.Sprintf("expecting int StatusMask attribute, got %T", mask))
				}
				options.StatusMask = int(maskval)
			}
			mask, ok = opts["GetMask"]
			if ok {
				maskval, ok := numInterfaceToInt(mask)
				if !ok {

					return mkOttoErr(&z, fmt.Sprintf("expecting int GetMask attribute, got %T", mask))
				}
				options.GetMask = int(maskval)
			}
		}
		req := GetReq{H: args[0].value.(Hash), StatusMask: options.StatusMask, GetMask: options.GetMask}
		var r interface{}
		r, err = NewGetAction(req, &options).Do(h)
		mask := options.GetMask
		if mask == GetMaskDefault {
			mask = GetMaskEntry
		}
		if err == nil {
			getResp := r.(GetResp)
			var singleValueReturn bool
			if mask&GetMaskEntry != 0 {
				if GetMaskEntry == mask {
					singleValueReturn = true
					result, err = z.vm.ToValue(getResp.Entry)
				}
			}
			if mask&GetMaskEntryType != 0 {
				if GetMaskEntryType == mask {
					singleValueReturn = true
					result, err = z.vm.ToValue(getResp.EntryType)
				}
			}
			if mask&GetMaskSources != 0 {
				if GetMaskSources == mask {
					singleValueReturn = true
					result, err = z.vm.ToValue(getResp.Sources)
				}
			}
			if err == nil && !singleValueReturn {
				respObj := make(map[string]interface{})
				if mask&GetMaskEntry != 0 {
					respObj["Entry"] = getResp.Entry
				}
				if mask&GetMaskEntryType != 0 {
					respObj["EntryType"] = getResp.EntryType
				}
				if mask&GetMaskEntryType != 0 {
					respObj["Sources"] = getResp.Sources
				}
				result, err = z.vm.ToValue(respObj)
			}
			return
		}

		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		panic("Shouldn't get here!")
	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("update", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionMod{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		entryType := args[0].value.(string)
		entryStr := args[1].value.(string)
		replaces := args[2].value.(Hash)

		entry := GobEntry{C: entryStr}
		resp, err := NewModAction(entryType, &entry, replaces).Do(h)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		var entryHash Hash
		if resp != nil {
			entryHash = resp.(Hash)
		}
		result, _ = z.vm.ToValue(entryHash.String())

		return

	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("remove", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionDel{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		entry := DelEntry{
			Hash:    args[0].value.(Hash),
			Message: args[1].value.(string),
		}
		header, err := h.chain.GetEntryHeader(entry.Hash)
		if err == nil {
			resp, err := NewDelAction(header.Type, entry).Do(h)
			if err == nil {
				var entryHash Hash
				if resp != nil {
					entryHash = resp.(Hash)
				}
				result, _ = z.vm.ToValue(entryHash.String())
				return
			}
		}
		result = mkOttoErr(&z, err.Error())
		return

	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("getLink", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionGetLink{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return z.vm.MakeCustomError("HolochainError", err.Error())
		}
		base := args[0].value.(Hash)
		tag := args[1].value.(string)

		l := len(call.ArgumentList)
		options := GetLinkOptions{Load: false, StatusMask: StatusLive}
		if l == 3 {
			opts := args[2].value.(map[string]interface{})
			load, ok := opts["Load"]
			if ok {
				loadval, ok := load.(bool)
				if !ok {
					return mkOttoErr(&z, fmt.Sprintf("expecting boolean Load attribute in object, got %T", load))
				}
				options.Load = loadval
			}
			mask, ok := opts["StatusMask"]
			if ok {
				maskval, ok := numInterfaceToInt(mask)
				if !ok {
					return mkOttoErr(&z, fmt.Sprintf("expecting int StatusMask attribute in object, got %T", mask))
				}
				options.StatusMask = int(maskval)
			}
		}
		var response interface{}

		response, err = NewGetLinkAction(&LinkQuery{Base: base, T: tag, StatusMask: options.StatusMask}, &options).Do(h)
		Debugf("RESPONSE:%v\n", response)

		if err == nil {
			result, err = z.vm.ToValue(response)
		} else {
			result = mkOttoErr(&z, err.Error())
		}

		return
	})
	if err != nil {
		return nil, err
	}

	l := JSLibrary
	if h != nil {
		l += fmt.Sprintf(`var App = {Name:"%s",DNA:{Hash:"%s"},Agent:{Hash:"%s",String:"%s"},Key:{Hash:"%s"}};`, h.Name, h.dnaHash, h.agentHash, h.Agent().Name(), peer.IDB58Encode(h.id))
	}
	_, err = z.Run(l + code)
	if err != nil {
		return
	}
	n = &z
	return
}

// Run executes javascript code
func (z *JSNucleus) Run(code string) (result *otto.Value, err error) {
	v, err := z.vm.Run(code)
	if err != nil {
		err = errors.New("JS exec error: " + err.Error())
		return
	}
	z.lastResult = &v
	return
}
