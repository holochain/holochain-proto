// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// JSRibosome implements a javascript use of the Ribosome interface

package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	"github.com/robertkrimen/otto"
	"strings"
	"time"
)

const (
	JSRibosomeType = "js"
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
		`,Bridge:{From:` + BridgeFromStr +
		`,To:` + BridgeToStr +
		"}" +
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

func mkOttoErr(jsr *JSRibosome, msg string) otto.Value {
	return jsr.vm.MakeCustomError("HolochainError", msg)
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

// NewJSRibosome factory function to build a javascript execution environment for a zome
func NewJSRibosome(h *Holochain, zome *Zome) (n Ribosome, err error) {
	jsr := JSRibosome{
		h:    h,
		zome: zome,
		vm:   otto.New(),
	}

	err = jsr.vm.Set("property", func(call otto.FunctionCall) otto.Value {
		a := &ActionProperty{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}

		a.prop = args[0].value.(string)

		var p interface{}
		p, err = a.Do(h)
		if err != nil {
			return otto.UndefinedValue()
		}
		result, _ := jsr.vm.ToValue(p)
		return result
	})
	if err != nil {
		return nil, err
	}

	err = jsr.vm.Set("debug", func(call otto.FunctionCall) otto.Value {
		a := &ActionDebug{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		a.msg = args[0].value.(string)
		a.Do(h)
		return otto.UndefinedValue()
	})

	err = jsr.vm.Set("makeHash", func(call otto.FunctionCall) otto.Value {
		a := &ActionMakeHash{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		a.entryType = args[0].value.(string)
		a.entry = &GobEntry{C: args[1].value.(string)}
		var r interface{}
		r, err = a.Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		var entryHash Hash
		if r != nil {
			entryHash = r.(Hash)
		}
		result, _ := jsr.vm.ToValue(entryHash.String())
		return result
	})

	//===========================================================================
	err = jsr.vm.Set("getBridges", func(call otto.FunctionCall) otto.Value {
		a := &ActionGetBridges{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		r, err := a.Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		var code string
		for i, b := range r.([]Bridge) {
			if i > 0 {
				code += ","
			}
			if b.Side == BridgeTo {
				code += fmt.Sprintf(`{Side:%d,Token:"%s"}`, b.Side, b.Token)
			} else {
				code += fmt.Sprintf(`{Side:%d,ToApp:"%s"}`, b.Side, b.ToApp.String())
			}
		}
		code = "[" + code + "]"
		object, _ := jsr.vm.Object(code)
		result, _ := jsr.vm.ToValue(object)

		return result
	})

	//===========================================================================
	err = jsr.vm.Set("sign", func(call otto.FunctionCall) otto.Value {
		a := &ActionSign{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}

		a.doc = []byte(args[0].value.(string))
		var r interface{}
		r, err = a.Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		var signature []byte
		if r != nil {
			signature = r.([]byte)
		}
		result, _ := jsr.vm.ToValue(string(signature))
		return result
	})

	err = jsr.vm.Set("verifySignature", func(call otto.FunctionCall) otto.Value {

		a := &ActionVerifySignature{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		a.signature = args[0].value.(string)
		a.data = args[1].value.(string)
		a.pubKey = args[2].value.(string)
		var r bool
		r, err = a.Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		var result otto.Value
		result, err = jsr.vm.ToValue(r)

		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		return result
	})

	//============================================================================

	err = jsr.vm.Set("send", func(call otto.FunctionCall) otto.Value {
		a := &ActionSend{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}

		a.to, err = peer.IDB58Decode(args[0].value.(Hash).String())
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		msg := args[1].value.(map[string]interface{})
		var j []byte
		j, err = json.Marshal(msg)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
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
					return mkOttoErr(&jsr, "callback option requires Function")
				}
				callback.Function = v.(string)
				v, ok = cbmap.(map[string]interface{})["ID"]
				if !ok {
					return mkOttoErr(&jsr, "callback option requires ID")
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
		r, err = a.Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		var result otto.Value
		result, err = jsr.vm.ToValue(r)

		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		return result
	})

	err = jsr.vm.Set("call", func(call otto.FunctionCall) otto.Value {
		a := &ActionCall{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		a.zome = args[0].value.(string)
		var zome *Zome
		zome, err = h.GetZome(a.zome)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		a.function = args[1].value.(string)
		var fn *FunctionDef
		fn, err = zome.GetFunctionDef(a.function)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		if fn.CallingType == JSON_CALLING {
			if !call.ArgumentList[2].IsObject() {
				return mkOttoErr(&jsr, "function calling type requires object argument type")
			}
		}
		a.args = args[2].value.(string)

		var r interface{}
		r, err = a.Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		var result otto.Value
		result, err = jsr.vm.ToValue(r)

		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		return result
	})

	err = jsr.vm.Set("bridge", func(call otto.FunctionCall) otto.Value {
		a := &ActionBridge{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		hash := args[0].value.(Hash)
		a.token, a.url, err = h.GetBridgeToken(hash)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}

		a.zome = args[1].value.(string)
		a.function = args[2].value.(string)
		a.args = args[3].value.(string)

		var r interface{}
		r, err = a.Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		var result otto.Value
		result, err = jsr.vm.ToValue(r)

		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		return result
	})

	err = jsr.vm.Set("commit", func(call otto.FunctionCall) otto.Value {
		var a Action = &ActionCommit{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}

		entryType := args[0].value.(string)
		entryStr := args[1].value.(string)
		var r interface{}
		entry := GobEntry{C: entryStr}
		r, err = NewCommitAction(entryType, &entry).Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		var entryHash Hash
		if r != nil {
			entryHash = r.(Hash)
		}

		result, _ := jsr.vm.ToValue(entryHash.String())
		return result
	})
	if err != nil {
		return nil, err
	}

	err = jsr.vm.Set("query", func(call otto.FunctionCall) otto.Value {
		a := &ActionQuery{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}

		if len(call.ArgumentList) == 1 {
			options := QueryOptions{}
			var j []byte
			j, err = json.Marshal(args[0].value)
			if err != nil {
				return mkOttoErr(&jsr, err.Error())
			}
			jsr.h.Debugf("Query options: %s", string(j))
			err = json.Unmarshal(j, &options)
			if err != nil {
				return mkOttoErr(&jsr, err.Error())
			}
			a.options = &options
		}
		r, err := a.Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		qr := r.([]QueryResult)

		defs := make(map[string]*EntryDef)
		var code string
		for i, result := range qr {
			if i > 0 {
				code += ","
			}
			var entryCode, hashCode, headerCode string
			var returnCount int
			if a.options.Return.Hashes {
				returnCount += 1
				hashCode = `"` + result.Header.EntryLink.String() + `"`
			}
			if a.options.Return.Headers {
				returnCount += 1
				headerCode = fmt.Sprintf(
					`{Type:"%s",Time:"%v",EntryLink:"%s",HeaderLink:"%s",TypeLink:"%s",}`,
					jsSanitizeString(result.Header.Type),
					result.Header.Time,
					result.Header.EntryLink.String(),
					result.Header.HeaderLink.String(),
					result.Header.TypeLink.String(),
				)
			}
			if a.options.Return.Entries {
				returnCount += 1

				var def *EntryDef
				var ok bool
				def, ok = defs[result.Header.Type]
				if !ok {
					_, def, err = h.GetEntryDef(result.Header.Type)
					if err != nil {
						return mkOttoErr(&jsr, err.Error())
					}
					defs[result.Header.Type] = def
				}
				r := result.Entry.Content()
				switch def.DataFormat {
				case DataFormatRawJS:
					entryCode = r.(string)
				case DataFormatString:
					entryCode = fmt.Sprintf(`"%s"`, jsSanitizeString(r.(string)))
				case DataFormatLinks:
					fallthrough
				case DataFormatJSON:
					entryCode = fmt.Sprintf(`JSON.parse("%s")`, jsSanitizeString(r.(string)))
				case DataFormatSysAgent:
					j, err := json.Marshal(r.(AgentEntry))
					if err != nil {
						return mkOttoErr(&jsr, err.Error())
					}
					entryCode = fmt.Sprintf(`JSON.parse("%s")`, jsSanitizeString(string(j)))
				default:
					return mkOttoErr(&jsr, "data format not implemented: "+def.DataFormat)
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
		results, _ := jsr.vm.ToValue(object)
		return results
	})

	err = jsr.vm.Set("get", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionGet{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
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
					return mkOttoErr(&jsr, fmt.Sprintf("expecting int StatusMask attribute, got %T", mask))
				}
				options.StatusMask = int(maskval)
			}
			mask, ok = opts["GetMask"]
			if ok {
				maskval, ok := numInterfaceToInt(mask)
				if !ok {

					return mkOttoErr(&jsr, fmt.Sprintf("expecting int GetMask attribute, got %T", mask))
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
		if err == nil {
			getResp := r.(GetResp)
			var singleValueReturn bool
			if mask&GetMaskEntry != 0 {
				if GetMaskEntry == mask {
					singleValueReturn = true
					result, err = jsr.vm.ToValue(getResp.Entry.Content())
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
					respObj["Entry"] = getResp.Entry.Content()
				}
				if mask&GetMaskEntryType != 0 {
					respObj["EntryType"] = getResp.EntryType
				}
				if mask&GetMaskSources != 0 {
					respObj["Sources"] = getResp.Sources
				}
				result, err = jsr.vm.ToValue(respObj)
			}
			return
		}

		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		panic("Shouldn't get here!")
	})
	if err != nil {
		return nil, err
	}

	err = jsr.vm.Set("update", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionMod{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		entryType := args[0].value.(string)
		entryStr := args[1].value.(string)
		replaces := args[2].value.(Hash)

		entry := GobEntry{C: entryStr}
		resp, err := NewModAction(entryType, &entry, replaces).Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		var entryHash Hash
		if resp != nil {
			entryHash = resp.(Hash)
		}
		result, _ = jsr.vm.ToValue(entryHash.String())

		return

	})
	if err != nil {
		return nil, err
	}

	err = jsr.vm.Set("updateAgent", func(call otto.FunctionCall) (result otto.Value) {
		a := &ActionModAgent{}
		//		var a Action = &ActionModAgent{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		opts := args[0].value.(map[string]interface{})
		id, idok := opts["Identity"]
		if idok {
			a.Identity = AgentIdentity(id.(string))
		}
		rev, revok := opts["Revocation"]
		if revok {
			a.Revocation = rev.(string)
		}

		resp, err := a.Do(h)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
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
				return mkOttoErr(&jsr, err.Error())
			}
		}

		// there's always a new agent entry
		_, err = jsr.vm.Run(`App.Agent.TopHash="` + h.agentTopHash.String() + `"`)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}

		// but not always a new identity to update
		if idok {
			_, err = jsr.vm.Run(`App.Agent.String="` + jsSanitizeString(id.(string)) + `"`)
			if err != nil {
				return mkOttoErr(&jsr, err.Error())
			}
		}

		result, _ = jsr.vm.ToValue(agentEntryHash.String())

		return
	})
	if err != nil {
		return nil, err
	}

	err = jsr.vm.Set("remove", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionDel{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&jsr, err.Error())
		}
		entry := DelEntry{
			Hash:    args[0].value.(Hash),
			Message: args[1].value.(string),
		}
		header, err := h.chain.GetEntryHeader(entry.Hash)
		if err == nil {
			var resp interface{}
			resp, err = NewDelAction(header.Type, entry).Do(h)
			if err == nil {
				var entryHash Hash
				if resp != nil {
					entryHash = resp.(Hash)
				}
				result, _ = jsr.vm.ToValue(entryHash.String())
				return
			}
		}
		result = mkOttoErr(&jsr, err.Error())
		return

	})
	if err != nil {
		return nil, err
	}

	err = jsr.vm.Set("getLinks", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionGetLinks{}
		args := a.Args()
		err := jsProcessArgs(&jsr, args, call.ArgumentList)
		if err != nil {
			return jsr.vm.MakeCustomError("HolochainError", err.Error())
		}
		base := args[0].value.(Hash)
		tag := args[1].value.(string)

		l := len(call.ArgumentList)
		options := GetLinksOptions{Load: false, StatusMask: StatusLive}
		if l == 3 {
			opts := args[2].value.(map[string]interface{})
			load, ok := opts["Load"]
			if ok {
				loadval, ok := load.(bool)
				if !ok {
					return mkOttoErr(&jsr, fmt.Sprintf("expecting boolean Load attribute in object, got %T", load))
				}
				options.Load = loadval
			}
			mask, ok := opts["StatusMask"]
			if ok {
				maskval, ok := numInterfaceToInt(mask)
				if !ok {
					return mkOttoErr(&jsr, fmt.Sprintf("expecting int StatusMask attribute in object, got %T", mask))
				}
				options.StatusMask = int(maskval)
			}
		}
		var response interface{}

		response, err = NewGetLinksAction(&LinkQuery{Base: base, T: tag, StatusMask: options.StatusMask}, &options).Do(h)

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
					_, def, err := h.GetEntryDef(th.EntryType)
					if err != nil {
						break
					}
					var entry string
					switch def.DataFormat {
					case DataFormatRawJS:
						entry = th.E
					case DataFormatRawZygo:
						fallthrough
					case DataFormatString:
						entry = `"` + jsSanitizeString(th.E) + `"`
					case DataFormatSysKey:
						entry = fmt.Sprintf("%v", th.E)
					case DataFormatSysAgent:
						fallthrough
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

		if err != nil {
			result = mkOttoErr(&jsr, err.Error())
		}

		return
	})
	if err != nil {
		return nil, err
	}

	l := JSLibrary
	if h != nil {
		l += fmt.Sprintf(`var App = {Name:"%s",DNA:{Hash:"%s"},Agent:{Hash:"%s",TopHash:"%s",String:"%s"},Key:{Hash:"%s"}};`, h.Name(), h.dnaHash, h.agentHash, h.agentTopHash, jsSanitizeString(string(h.Agent().Identity())), h.nodeIDStr)
	}
	_, err = jsr.Run(l + zome.Code)
	if err != nil {
		return
	}
	n = &jsr
	return
}

// Run executes javascript code
func (jsr *JSRibosome) Run(code string) (result interface{}, err error) {
	v, err := jsr.vm.Run(code)
	if err != nil {
		err = fmt.Errorf("Error executing JavaScript: " + err.Error())
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
