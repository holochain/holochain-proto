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
	_ "math"
	"strings"
	"time"
)

const (
	JSNucleusType = "js"
)

type JSNucleus struct {
	vm         *otto.Otto
	interfaces []Interface
	lastResult *otto.Value
}

// Name returns the string value under which this nucleus is registered
func (z *JSNucleus) Type() string { return JSNucleusType }

// ChainReqires runs the application requires function
// this function gets called so that the holochain library can confirm that it is capable of
// servicing the needs of the application.
func (z *JSNucleus) ChainRequires() (err error) {
	v, err := z.vm.Run(`requires()`)
	if err != nil {
		if err.Error() == "ReferenceError: 'requires' is not defined" {
			ChangeRequires.Log()
			err = nil
		} else {
			err = fmt.Errorf("Error executing requires: %v", err)
		}
		return
	}
	if v.IsObject() {
		var vObj otto.Value
		vObj, err = v.Object().Get("version")
		if err == nil {
			var version int64
			version, err = vObj.ToInteger()
			if err == nil {
				if version > int64(Version) {
					err = fmt.Errorf("Zome requires version %d", version)
				}
			}
		}

	} else {
		err = fmt.Errorf("require should return an object, got: %v", v)
	}
	return
}

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

// ValidateEntry checks the contents of an entry against the validation rules
// this is the zgo implementation
func (z *JSNucleus) ValidateEntry(d *EntryDef, entry Entry, props *ValidationProps) (err error) {
	c := entry.Content().(string)
	var e string
	switch d.DataFormat {
	case DataFormatRawJS:
		e = c
	case DataFormatString:
		e = "\"" + jsSanitizeString(c) + "\""
	case DataFormatJSON:
		e = fmt.Sprintf(`JSON.parse("%s")`, jsSanitizeString(c))
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
	code := fmt.Sprintf(`validate("%s",%s,JSON.parse("%s"))`, d.Name, e, jsSanitizeString(string(b)))
	v, err := z.vm.Run(code)
	if err != nil {
		err = fmt.Errorf("Error executing validate: %v", err)
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
				err = fmt.Errorf("Invalid entry: %v", entry.Content())
			}
		}
	} else {
		err = fmt.Errorf("validate should return boolean, got: %v", v)
	}
	return
}

// GetInterface returns an Interface of the given name
func (z *JSNucleus) GetInterface(iface string) (i *Interface, err error) {
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
func (z *JSNucleus) Interfaces() (i []Interface) {
	i = z.interfaces
	return
}

// expose registers an interfaces defined in the DNA for calling by external clients
// (you should probably never need to call this function as it is called by the DNA's expose functions)
func (z *JSNucleus) expose(iface Interface) (err error) {
	z.interfaces = append(z.interfaces, iface)
	return
}

const (
	JSLibrary = `var HC={STRING:0,JSON:1};version=` + `"` + VersionStr + `";`
)

// jsSanatizeString makes sure all quotes are quoted and returns are removed
func jsSanitizeString(s string) string {
	s0 := strings.Replace(s, "\n", "", -1)
	s1 := strings.Replace(s0, "\r", "", -1)
	s2 := strings.Replace(s1, "\"", "\\\"", -1)
	return s2
}

// Call calls the zygo function that was registered with expose
func (z *JSNucleus) Call(iface string, params interface{}) (result interface{}, err error) {
	var i *Interface
	i, err = z.GetInterface(iface)
	if err != nil {
		return
	}
	var code string
	switch i.Schema {
	case STRING:
		code = fmt.Sprintf(`%s("%s");`, iface, jsSanitizeString(params.(string)))
	case JSON:
		if params.(string) == "" {
			code = fmt.Sprintf(`JSON.stringify(%s());`, iface)
		} else {
			p := jsSanitizeString(params.(string))
			code = fmt.Sprintf(`JSON.stringify(%s(JSON.parse("%s")));`, iface, p)
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

// NewJSNucleus builds a javascript execution environment with user specified code
func NewJSNucleus(h *Holochain, code string) (n Nucleus, err error) {
	var z JSNucleus
	z.vm = otto.New()

	err = z.vm.Set("property", func(call otto.FunctionCall) otto.Value {
		prop, _ := call.Argument(0).ToString()

		p, err := h.GetProperty(prop)
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
		msg, _ := call.Argument(0).ToString()
		h.config.Loggers.App.p(msg)
		return otto.UndefinedValue()
	})

	err = z.vm.Set("expose", func(call otto.FunctionCall) otto.Value {
		fnName, _ := call.Argument(0).ToString()
		schema, _ := call.Argument(1).ToInteger()
		i := Interface{Name: fnName, Schema: InterfaceSchemaType(schema)}
		err = z.expose(i)
		if err != nil {
			return z.vm.MakeCustomError("HolochainError", err.Error())
		}
		return otto.UndefinedValue()
	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("commit", func(call otto.FunctionCall) otto.Value {
		entryType, _ := call.Argument(0).ToString()
		var entry string
		v := call.Argument(1)

		if v.IsString() {
			entry, _ = v.ToString()
		} else if v.IsObject() {
			v, _ = z.vm.Call("JSON.stringify", nil, v)
			entry, _ = v.ToString()
		} else {
			return z.vm.MakeCustomError("HolochainError", "commit expected string as second argument")
		}

		e := GobEntry{C: entry}
		var l int
		var hash Hash
		var header *Header
		l, hash, header, err = h.chain.PrepareHeader(h.hashSpec, time.Now(), entryType, &e, h.agent.PrivKey())
		if err != nil {
			return z.vm.MakeCustomError("HolochainError", err.Error())
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
			return z.vm.MakeCustomError("HolochainError", err.Error())
		}

		result, _ := z.vm.ToValue(header.EntryLink.String())
		return result
	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("put", func(call otto.FunctionCall) otto.Value {
		v := call.Argument(0)
		var hashstr string

		if v.IsString() {
			hashstr, _ = v.ToString()
		} else {
			return z.vm.MakeCustomError("HolochainError", "put expected string as argument")
		}

		var key Hash
		key, err = NewHash(hashstr)
		if err == nil {
			err = h.dht.SendPut(key)
		}

		if err != nil {
			return z.vm.MakeCustomError("HolochainError", err.Error())
		}

		return otto.UndefinedValue()
	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("get", func(call otto.FunctionCall) (result otto.Value) {
		v := call.Argument(0)
		var hashstr string

		if v.IsString() {
			hashstr, _ = v.ToString()
		} else {
			return z.vm.MakeCustomError("HolochainError", "get expected string as argument")
		}

		var key Hash
		key, err = NewHash(hashstr)
		if err == nil {
			var response interface{}
			response, err = h.dht.SendGet(key)
			if err == nil {
				switch t := response.(type) {
				case *GobEntry:
					result, err = z.vm.ToValue(t)
					return
					// @TODO what about if the hash was of a header??
				default:
					err = fmt.Errorf("unexpected response type from SendGet: %v", t)
				}

			}
		}

		if err != nil {
			result = z.vm.MakeCustomError("HolochainError", err.Error())
			return
		}
		panic("Shouldn't get here!")
	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("putmeta", func(call otto.FunctionCall) otto.Value {
		hashstr, _ := call.Argument(0).ToString()
		metahashstr, _ := call.Argument(1).ToString()
		typestr, _ := call.Argument(2).ToString()

		var key Hash
		key, err = NewHash(hashstr)
		if err == nil {
			var metakey Hash
			metakey, err = NewHash(metahashstr)
			if err == nil {
				err = h.dht.SendPutMeta(MetaReq{O: key, M: metakey, T: typestr})
			}
		}

		if err != nil {
			return z.vm.MakeCustomError("HolochainError", err.Error())
		}

		return otto.UndefinedValue()
	})
	if err != nil {
		return nil, err
	}

	err = z.vm.Set("getmeta", func(call otto.FunctionCall) (result otto.Value) {
		hashstr, _ := call.Argument(0).ToString()
		typestr, _ := call.Argument(1).ToString()

		var key Hash
		key, err = NewHash(hashstr)
		var response interface{}
		if err == nil {
			response, err = h.dht.SendGetMeta(MetaQuery{H: key, T: typestr})
			if err == nil {
				result, err = z.vm.ToValue(response)
			}
		}

		if err != nil {
			return z.vm.MakeCustomError("HolochainError", err.Error())
		}

		return
	})
	if err != nil {
		return nil, err
	}
	l := JSLibrary
	if h != nil {
		l += fmt.Sprintf(`var App = {DNAHash:"%s",AgentHash:"%s",AgentStr:"%s",KeyHash:"%s"};`, h.dnaHash, h.agentHash, h.Agent().Name(), peer.IDB58Encode(h.id))
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
