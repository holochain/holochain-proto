// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// JSRibosome implements a javascript use of the Ribosome interface

package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/HC-Interns/holochain-proto/hash"
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
		`Migrate:"` + MigrateEntryType + `"` +
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
		`Migrate:{Close:"` + MigrateEntryTypeClose + `",Open:"` + MigrateEntryTypeOpen + `"}` +
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

func makeOttoObjectFromGetResp(h *Holochain, vm *otto.Otto, getResp *GetResp) (result interface{}, err error) {
	_, def, err := h.GetEntryDef(getResp.EntryType)
	if err != nil {
		return
	}
	if def.DataFormat == DataFormatJSON {
		json := getResp.Entry.Content().(string)
		code := `(` + json + `)`
		result, err = vm.Object(code)
	} else {
		result = getResp.Entry.Content().(string)
	}
	return
}


// NewJSRibosome factory function to build a javascript execution environment for a zome
func Setup(jsr *JSRibosome, h *Holochain, zome *Zome) (err error) {
	jsr.h = h
	jsr.zome = zome
	funcs := JSRibosomeFuncs(h, zome, jsr.vm)

	var fnPrefix string
	var returnErrors bool
	val, ok := zome.Config["ErrorHandling"]
	if ok {
		var errHandling string
		errHandling, ok = val.(string)
		if !ok {
			err = errors.New("Expected ErrorHandling config value to be string")
			return err
		}
		switch errHandling {
		case ErrHandlingThrowErrorsStr:
		case ErrHandlingReturnErrorsStr:
			returnErrors = true
		default:
			err = fmt.Errorf("Expected ErrorHandling config value to be '%s' or '%s', was: '%s'", ErrHandlingThrowErrorsStr, ErrHandlingReturnErrorsStr, errHandling)
			return err
		}

	}
	if !returnErrors {
		fnPrefix = "__"
	}

	for name, data := range funcs {
		wfn := makeJSFN(jsr, name, data)
		err = jsr.vm.Set(fnPrefix+name, wfn)
		if err != nil {
			return err
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
	return
}

func NewJSRibosome(h *Holochain, zome *Zome) (Ribosome, error) {
  jsr := BareJSRibosome()
  err := Setup(&jsr, h, zome)
  if err != nil {
  	return nil, err
  }
  return &jsr, err
}

func BareJSRibosome() (jsr JSRibosome) {
  jsr = JSRibosome{
    h:    nil,
    zome: nil,
    vm:   otto.New(),
  }
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

func GetVM(jsr *JSRibosome) *otto.Otto {
	return jsr.vm
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

//////////////////////////////
//
// from https://github.com/robertkrimen/natto/blob/master/natto.go

type _timer struct {
	timer    *time.Timer
	duration time.Duration
	interval bool
	call     otto.FunctionCall
}

// RunWithTimers will execute the given JavaScript in the usual Ribosome runtime,
// but including implementations of some timer-related JS functions.
// The Otto VM will continue to run until all timers have finished executing (if any).
// The VM has the following functions available:
//
//      <timer> = setTimeout(<function>, <delay>, [<arguments...>])
//      <timer> = setInterval(<function>, <delay>, [<arguments...>])
//      clearTimeout(<timer>)
//      clearInterval(<timer>)
//
func RunWithTimers(vm *otto.Otto, src string) (result interface{}, err error) {
	registry := map[*_timer]*_timer{}
	ready := make(chan *_timer)

	newTimer := func(call otto.FunctionCall, interval bool) (*_timer, otto.Value) {
		delay, _ := call.Argument(1).ToInteger()
		if 0 >= delay {
			delay = 1
		}

		timer := &_timer{
			duration: time.Duration(delay) * time.Millisecond,
			call:     call,
			interval: interval,
		}
		registry[timer] = timer

		timer.timer = time.AfterFunc(timer.duration, func() {
			ready <- timer
		})

		value, err := call.Otto.ToValue(timer)
		if err != nil {
			panic(err)
		}

		return timer, value
	}

	setTimeout := func(call otto.FunctionCall) otto.Value {
		_, value := newTimer(call, false)
		return value
	}
	vm.Set("setTimeout", setTimeout)

	setInterval := func(call otto.FunctionCall) otto.Value {
		_, value := newTimer(call, true)
		return value
	}
	vm.Set("setInterval", setInterval)

	clearTimeout := func(call otto.FunctionCall) otto.Value {
		timer, _ := call.Argument(0).Export()
		if timer, ok := timer.(*_timer); ok {
			timer.timer.Stop()
			delete(registry, timer)
		}
		return otto.UndefinedValue()
	}
	vm.Set("clearTimeout", clearTimeout)
	vm.Set("clearInterval", clearTimeout)

	result, err = vm.Run(src)
	if err != nil {
		return result, err
	}

	for {
		select {
		case timer := <-ready:
			var arguments []interface{}
			if len(timer.call.ArgumentList) > 2 {
				tmp := timer.call.ArgumentList[2:]
				arguments = make([]interface{}, 2+len(tmp))
				for i, value := range tmp {
					arguments[i+2] = value
				}
			} else {
				arguments = make([]interface{}, 1)
			}
			arguments[0] = timer.call.ArgumentList[0]
			_, err := vm.Call(`Function.call.call`, nil, arguments...)
			if err != nil {
				for _, timer := range registry {
					timer.timer.Stop()
					delete(registry, timer)
					return result, err
				}
			}
			if timer.interval {
				timer.timer.Reset(timer.duration)
			} else {
				delete(registry, timer)
			}
		default:
			// Escape valve!
			// If this isn't here, we deadlock...
		}
		if len(registry) == 0 {
			break
		}
	}

	return result, nil
}
