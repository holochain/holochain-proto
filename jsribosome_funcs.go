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
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/robertkrimen/otto"
)

func JSRibosomeFuncs(h *Holochain, zome *Zome, jsr JSRibosome) map[string]fnData {
	return map[string]fnData{
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
				DNAHash := args[1].value.(Hash)
				Key := args[2].value.(Hash)
				Data := args[3].value.(string)
				var r interface{}
				f.action.entry.Type = migrationType
				f.action.entry.DNAHash = DNAHash
				f.action.entry.Key = Key
				f.action.entry.Data = Data
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
}
