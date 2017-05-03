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

	hdr := fmt.Sprintf(
		`{"EntryLink":"%s","Type":"%s","Time":"%s"}`,
		header.EntryLink.String(),
		header.Type,
		header.Time.UTC().Format(time.RFC3339),
	)
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
		args = fmt.Sprintf(`"%s","%s"`, t.hash.String(), t.newHash.String())
	case *ActionDel:
		args = fmt.Sprintf(`"%s"`, t.hash.String())
	case *ActionLink:
		var j []byte
		j, err = json.Marshal(t.links)
		if err == nil {
			args = fmt.Sprintf(`"%s",JSON.parse("%s")`, t.validationBase.String(), jsSanitizeString(string(j)))
		}
	case *ActionDelLink:
		args = fmt.Sprintf(`"%s","%s","%s"`, t.link.Base.String(), t.link.Link.String(), t.link.Tag)
	default:
		err = fmt.Errorf("can't prepare args for %T: ", t)
		return
	}
	return
}

func buildJSValidateAction(action Action, def *EntryDef, sources []string) (code string, err error) {
	fnName := "validate" + strings.Title(action.Name())
	var args string
	args, err = prepareJSValidateArgs(action, def)
	if err != nil {
		return
	}
	srcs := mkJSSources(sources)
	code = fmt.Sprintf(`%s("%s",%s,%s)`, fnName, def.Name, args, srcs)

	return
}

// ValidateAction builds the correct validation function based on the action an calls it
func (z *JSNucleus) ValidateAction(action Action, def *EntryDef, sources []string) (err error) {
	var code string
	code, err = buildJSValidateAction(action, def, sources)
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
	JSLibrary = `var HC={Version:` + `"` + VersionStr +
		`",Status:{Live:` + StatusLiveVal +
		`,Rejected:` + StatusRejectedVal +
		`,Deleted:` + StatusDeletedVal +
		`,Modified:` + StatusModifiedVal +
		`,Any:` + StatusAnyVal +
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

func jsProcessArgs(z *JSNucleus, args []Arg, oArgs []otto.Value) (err error) {
	err = checkArgCount(args, len(oArgs))
	if err != nil {
		return err
	}

	// check arg types
	for i, a := range oArgs {
		switch args[i].Type {
		case StringArg:
			if a.IsString() {
				args[i].value, _ = a.ToString()
			} else {
				return argErr("string", i+1, args[i])
			}
		case HashArg:
			if a.IsString() {
				str, _ := a.ToString()
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
			if a.IsNumber() {
				integer, err := a.ToInteger()
				if err != nil {
					return err
				}
				args[i].value = integer
			} else {
				return argErr("int", i+1, args[i])
			}
		case BoolArg:
			if a.IsBoolean() {
				boolean, err := a.ToBoolean()
				if err != nil {
					return err
				}
				args[i].value = boolean
			} else {
				return argErr("boolean", i+1, args[i])
			}
		case EntryArg:
			if a.IsString() {
				str, err := a.ToString()
				if err != nil {
					return err
				}
				args[i].value = str
			} else if a.IsObject() {
				v, err := z.vm.Call("JSON.stringify", nil, a)
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
			if a.IsObject() {
				m, err := a.Export()
				if err != nil {
					return err
				}
				args[i].value = m
			} else {
				return argErr("object", i+1, args[i])
			}
		case ToStrArg:
			var str string
			if a.IsObject() {
				v, err := z.vm.Call("JSON.stringify", nil, a)
				if err != nil {
					return err
				}
				str, err = v.ToString()
				if err != nil {
					return err
				}
			} else {
				str, _ = a.ToString()
			}
			args[i].value = str
		}
	}

	return
}

func mkOttoErr(z *JSNucleus, msg string) otto.Value {
	return z.vm.MakeCustomError("HolochainError", msg)
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

	err = z.vm.Set("commit", func(call otto.FunctionCall) otto.Value {
		var a Action = &ActionCommit{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}

		entryType := args[0].value.(string)
		entry := args[1].value.(string)
		var r interface{}
		e := GobEntry{C: entry}
		r, err = NewCommitAction(entryType, &e).Do(h)
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

		req := GetReq{H: args[0].value.(Hash), StatusMask: StatusDefault}
		if len(call.ArgumentList) == 2 {
			req.StatusMask = int(args[1].value.(int64))
		}

		entry, err := NewGetAction(req).Do(h)
		if err == nil {
			t := entry.(*GobEntry)
			result, err = z.vm.ToValue(t)
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

	err = z.vm.Set("mod", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionMod{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		hash := args[0].value.(Hash)
		newHash := args[1].value.(Hash)

		resp, err := NewModAction(hash, newHash).Do(h)
		if err == nil {
			result, err = z.vm.ToValue(resp)
			return
		}
		result = mkOttoErr(&z, err.Error())
		return

	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("del", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionDel{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			return mkOttoErr(&z, err.Error())
		}
		hash := args[0].value.(Hash)

		resp, err := NewDelAction(hash).Do(h)
		if err == nil {
			result, err = z.vm.ToValue(resp)
			return
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
				maskval, ok := mask.(int64)
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

	err = z.vm.Set("delLink", func(call otto.FunctionCall) (result otto.Value) {
		var a Action = &ActionDelLink{}
		args := a.Args()
		err := jsProcessArgs(&z, args, call.ArgumentList)
		if err != nil {
			result = mkOttoErr(&z, err.Error())
		}
		base := args[0].value.(Hash)
		link := args[1].value.(Hash)
		tag := args[2].value.(string)

		var response interface{}
		response, err = NewDelLinkAction(&DelLinkReq{Base: base, Link: link, Tag: tag}).Do(h)

		if err == nil {
			result, err = z.vm.ToValue(response)
		} else {
			result = mkOttoErr(&z, err.Error())
		}

		return
	})

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
