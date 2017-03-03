// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// JSNucleus implements a javascript use of the Nucleus interface

package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/robertkrimen/otto"
	_ "math"
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

// ValidateEntry checks the contents of an entry against the validation rules
// this is the zgo implementation
func (z *JSNucleus) ValidateEntry(d *EntryDef, entry interface{}) (err error) {
	var e string
	switch d.DataFormat {
	case "js":
		e = entry.(string)
	case "string":
		e = "\"" + sanitizeString(entry.(string)) + "\""
	case "JSON":
		e = fmt.Sprintf(`JSON.parse("%s")`, sanitizeString(entry.(string)))
	default:
		err = errors.New("data format not implemented: " + d.DataFormat)
		return
	}
	v, err := z.vm.Run(fmt.Sprintf(`validate("%s",%s)`, d.Name, e))
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
				err = fmt.Errorf("Invalid entry: %v", entry)
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
	JSLibrary = `var _STRING=0; var _JSON=1;version=` + `"` + Version + `";`
)

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
		code = fmt.Sprintf(`%s("%s");`, iface, sanitizeString(params.(string)))
	case JSON:
		code = fmt.Sprintf(`JSON.stringify(%s(JSON.parse("%s")));`, iface, sanitizeString(params.(string)))
	default:
		err = errors.New("params type not implemented")
		return
	}
	log.Debugf("JS Call:\n%s", code)
	var v otto.Value
	v, err = z.vm.Run(code)
	if err == nil {
		if v.IsObject() && v.Class() == "Error" {
			log.Debugf("JS Error:\n%v", v)
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
			entry, _ = v.ToString()
		} else {
			return z.vm.MakeCustomError("HolochainError", "commit expected string or object as second argument")
		}
		err = h.ValidateEntry(entryType, entry)
		var headerHash Hash

		if err == nil {
			e := GobEntry{C: entry}
			headerHash, _, err = h.NewEntry(time.Now(), entryType, &e)
		}
		if err != nil {
			return z.vm.MakeCustomError("HolochainError", err.Error())
		}

		result, _ := z.vm.ToValue(headerHash.String())
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
					// @TODO figure out encoding by entry type.
					var j []byte
					j, err = json.Marshal(t.C)
					if err == nil {
						result, err = z.vm.ToValue(string(j))
						if err == nil {
							return
						}
					}
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
				var j []byte
				j, err = json.Marshal(response)
				if err == nil {
					result, err = z.vm.ToValue(string(j))
				}
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

	_, err = z.Run(JSLibrary + code)
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
