// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	"reflect"
	"time"
)

type ArgType int8

// these constants define the argument types for actions, i.e. system functions callable
// from within nuclei
const (
	HashArg = iota
	StringArg
	EntryArg // special arg type for entries, can be a string or a hash
	IntArg
	BoolArg
	MapArg
	ToStrArg // special arg type that converts anything to a string, used for the debug action
	ArgsArg  // special arg type for arguments passed to the call action
)

type Arg struct {
	Name     string
	Type     ArgType
	Optional bool
	MapType  reflect.Type
	value    interface{}
}

// Action provides an abstraction for grouping all the aspects of a nucleus function, i.e.
// the validation,dht changing, etc
type Action interface {
	Name() string
	Do(h *Holochain) (response interface{}, err error)
	DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error)
	Args() []Arg
}

// CommittingAction provides an abstraction for grouping actions which carry Entry data
type CommittingAction interface {
	Name() string
	Do(h *Holochain) (response interface{}, err error)
	SysValidation(h *Holochain, d *EntryDef, sources []peer.ID) (err error)
	DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error)
	CheckValidationRequest(def *EntryDef) (err error)
	Args() []Arg
	EntryType() string
	Entry() Entry
}

// ValidatingAction provides an abstraction for grouping all the actions that participate in validation loop
type ValidatingAction interface {
	Name() string
	Do(h *Holochain) (response interface{}, err error)
	SysValidation(h *Holochain, d *EntryDef, sources []peer.ID) (err error)
	DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error)
	CheckValidationRequest(def *EntryDef) (err error)
	Args() []Arg
}

var NonDHTAction error = errors.New("Not a DHT action")
var NonCallableAction error = errors.New("Not a callable action")

func prepareSources(sources []peer.ID) (srcs []string) {
	srcs = make([]string, 0)
	for _, s := range sources {
		srcs = append(srcs, peer.IDB58Encode(s))
	}
	return
}

// ValidateAction runs the different phases of validating an action
func (h *Holochain) ValidateAction(a ValidatingAction, entryType string, pkg *Package, sources []peer.ID) (d *EntryDef, err error) {
	switch entryType {
	case DNAEntryType:
		//		panic("attempt to get validation response for DNA")
	case KeyEntryType:
		//		resp.Entry = TODO public key goes here
	case AgentEntryType:
	default:

		// validation actions for application defined entry types

		var z *Zome
		z, d, err = h.GetEntryDef(entryType)
		if err != nil {
			return
		}

		var vpkg *ValidationPackage
		vpkg, err = MakeValidationPackage(h, pkg)
		if err != nil {
			return
		}

		// run the action's system level validations
		err = a.SysValidation(h, d, sources)
		if err != nil {
			Debugf("Sys ValidateAction(%T) err:%v\n", a, err)
			return
		}

		// run the action's app level validations
		var n Nucleus
		n, err = h.makeNucleus(z)
		if err != nil {
			return
		}

		err = n.ValidateAction(a, d, vpkg, prepareSources(sources))
		if err != nil {
			Debugf("Nucleus ValidateAction(%T) err:%v\n", a, err)
		}
	}
	return
}

// GetValidationResponse check the validation request and builds the validation package based
// on the app's requirements
func (h *Holochain) GetValidationResponse(a ValidatingAction, hash Hash) (resp ValidateResponse, err error) {
	var entry Entry
	entry, resp.Type, err = h.chain.GetEntry(hash)
	if err == ErrHashNotFound {
		if hash.String() == peer.IDB58Encode(h.id) {
			resp.Type = KeyEntryType
			err = nil
		} else {
			return
		}
	} else if err != nil {
		return
	} else {
		resp.Entry = *(entry.(*GobEntry))
		var hd *Header
		hd, err = h.chain.GetEntryHeader(hash)
		if err != nil {
			return
		}
		resp.Header = *hd
	}
	switch resp.Type {
	case DNAEntryType:
		panic("attempt to get validation response for DNA")
	case KeyEntryType:
		//		resp.Entry = TODO public key goes here
	case AgentEntryType:
	default:
		// app defined entry types
		var def *EntryDef
		var z *Zome
		z, def, err = h.GetEntryDef(resp.Type)
		if err != nil {
			return
		}
		err = a.CheckValidationRequest(def)
		if err != nil {
			return
		}

		// get the packaging request from the app
		var n Nucleus
		n, err = h.makeNucleus(z)
		if err != nil {
			return
		}

		var req PackagingReq
		req, err = n.ValidatePackagingRequest(a, def)
		if err != nil {
			Debugf("Nucleus GetValidationPackage(%T) err:%v\n", a, err)
		}
		resp.Package, err = MakePackage(h, req)
	}
	return
}

// GetDHTReqAction generates an action from DHT request
// TODO this should be refactored into the Action interface
func (h *Holochain) GetDHTReqAction(msg *Message) (a Action, err error) {
	var t reflect.Type
	// TODO this can be refactored into Action
	switch msg.Type {
	case PUT_REQUEST:
		a = &ActionPut{}
		t = reflect.TypeOf(PutReq{})
	case GET_REQUEST:
		a = &ActionGet{}
		t = reflect.TypeOf(GetReq{})
	case MOD_REQUEST:
		a = &ActionMod{}
		t = reflect.TypeOf(ModReq{})
	case DEL_REQUEST:
		a = &ActionDel{}
		t = reflect.TypeOf(DelReq{})
	case LINK_REQUEST:
		a = &ActionLink{}
		t = reflect.TypeOf(LinkReq{})
	case GETLINK_REQUEST:
		a = &ActionGetLink{}
		t = reflect.TypeOf(LinkQuery{})
	default:
		err = fmt.Errorf("message type %d not in holochain-dht protocol", int(msg.Type))
	}
	if err == nil && reflect.TypeOf(msg.Body) != t {
		err = fmt.Errorf("Unexpected request body type '%T' in %s request", msg.Body, a.Name())
	}
	return
}

var ErrWrongNargs = errors.New("wrong number of arguments")

func checkArgCount(args []Arg, l int) (err error) {
	var min int
	for _, a := range args {
		if !a.Optional {
			min++
		}
	}
	if l < min || l > len(args) {
		err = ErrWrongNargs
	}
	return
}

func argErr(typeName string, index int, arg Arg) error {
	return fmt.Errorf("argument %d (%s) should be %s", index, arg.Name, typeName)
}

//------------------------------------------------------------
// Property

type ActionProperty struct {
	prop string
}

func NewPropertyAction(prop string) *ActionProperty {
	a := ActionProperty{prop: prop}
	return &a
}

func (a *ActionProperty) Name() string {
	return "property"
}

func (a *ActionProperty) Args() []Arg {
	return []Arg{{Name: "name", Type: StringArg}}
}

func (a *ActionProperty) Do(h *Holochain) (response interface{}, err error) {
	response, err = h.GetProperty(a.prop)
	return
}

//------------------------------------------------------------
// Debug

type ActionDebug struct {
	msg string
}

func NewDebugAction(msg string) *ActionDebug {
	a := ActionDebug{msg: msg}
	return &a
}

func (a *ActionDebug) Name() string {
	return "debug"
}

func (a *ActionDebug) Args() []Arg {
	return []Arg{{Name: "value", Type: ToStrArg}}
}

func (a *ActionDebug) Do(h *Holochain) (response interface{}, err error) {
	h.config.Loggers.App.p(a.msg)
	return
}

//------------------------------------------------------------
// MakeHash

type ActionMakeHash struct {
	entry Entry
}

func NewMakeHashAction(entry Entry) *ActionMakeHash {
	a := ActionMakeHash{entry: entry}
	return &a
}

func (a *ActionMakeHash) Name() string {
	return "makeHash"
}

func (a *ActionMakeHash) Args() []Arg {
	return []Arg{{Name: "entry", Type: EntryArg}}
}

func (a *ActionMakeHash) Do(h *Holochain) (response interface{}, err error) {
	var hash Hash
	hash, err = a.entry.Sum(h.hashSpec)
	if err != nil {
		return
	}
	response = hash
	return
}

//------------------------------------------------------------
// Call

type ActionCall struct {
	zome     string
	function string
	args     interface{}
}

func NewCallAction(zome string, function string, args interface{}) *ActionCall {
	a := ActionCall{zome: zome, function: function, args: args}
	return &a
}

func (a *ActionCall) Name() string {
	return "call"
}

func (a *ActionCall) Args() []Arg {
	return []Arg{{Name: "zome", Type: StringArg}, {Name: "function", Type: StringArg}, {Name: "args", Type: ArgsArg}}
}

func (a *ActionCall) Do(h *Holochain) (response interface{}, err error) {
	response, err = h.Call(a.zome, a.function, a.args, ZOME_EXPOSURE)
	return
}

//------------------------------------------------------------
// Get

type ActionGet struct {
	req     GetReq
	options *GetOptions
}

func NewGetAction(req GetReq, options *GetOptions) *ActionGet {
	a := ActionGet{req: req, options: options}
	return &a
}

func (a *ActionGet) Name() string {
	return "get"
}

func (a *ActionGet) Args() []Arg {
	return []Arg{{Name: "hash", Type: HashArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(GetOptions{}), Optional: true}}
}

func (a *ActionGet) Do(h *Holochain) (response interface{}, err error) {
	rsp, err := h.dht.Send(a.req.H, GET_REQUEST, a.req)
	if err != nil {

		// follow the modified hash
		if a.req.StatusMask == StatusDefault && err == ErrHashModified {
			var hash Hash
			hash, err = NewHash(rsp.(GetResp).FollowHash)
			if err != nil {
				return
			}
			req := GetReq{H: hash, StatusMask: StatusDefault, GetMask: a.options.GetMask}
			modResp, err := NewGetAction(req, a.options).Do(h)
			if err == nil {
				response = modResp
			}
		}
		return
	}
	switch t := rsp.(type) {
	case GetResp:
		response = t
	default:
		err = fmt.Errorf("expected GetResp response from GET_REQUEST, got: %T", t)
		return
	}
	return
}

func (a *ActionGet) SysValidation(h *Holochain, d *EntryDef, sources []peer.ID) (err error) {
	return
}

func (a *ActionGet) DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error) {
	var entryData []byte
	//var status int
	req := msg.Body.(GetReq)
	mask := req.GetMask
	if mask == GetMaskDefault {
		mask = GetMaskEntry
	}
	resp := GetResp{}
	entryData, resp.EntryType, resp.Sources, _, err = dht.get(req.H, req.StatusMask, req.GetMask)
	if err == nil {
		if (mask & GetMaskEntry) != 0 {
			var e GobEntry
			err = e.Unmarshal(entryData)
			if err != nil {
				return
			}
			resp.Entry = &e
		}
	} else {
		if err == ErrHashModified {
			resp.FollowHash = string(entryData)
		}
	}
	response = resp
	return
}

// doCommit adds an entry to the local chain after validating the action it's part of
func (h *Holochain) doCommit(a CommittingAction, change *StatusChange) (d *EntryDef, header *Header, entryHash Hash, err error) {

	entryType := a.EntryType()
	entry := a.Entry()
	var l int
	var hash Hash
	l, hash, header, err = h.chain.PrepareHeader(h.hashSpec, time.Now(), entryType, entry, h.agent.PrivKey(), change)
	if err != nil {
		return
	}
	//TODO	a.header = header
	d, err = h.ValidateAction(a, entryType, nil, []peer.ID{h.id})
	if err != nil {
		if err == ValidationFailedErr {
			err = fmt.Errorf("Invalid entry: %v", entry.Content())
		}
		return
	}
	err = h.chain.addEntry(l, hash, header, entry)
	if err != nil {
		return
	}
	entryHash = header.EntryLink
	return
}

//------------------------------------------------------------
// Commit

type ActionCommit struct {
	entryType string
	entry     Entry
	header    *Header
}

func NewCommitAction(entryType string, entry Entry) *ActionCommit {
	a := ActionCommit{entryType: entryType, entry: entry}
	return &a
}

func (a *ActionCommit) Entry() Entry {
	return a.entry
}

func (a *ActionCommit) EntryType() string {
	return a.entryType
}

func (a *ActionCommit) Name() string {
	return "commit"
}

func (a *ActionCommit) Args() []Arg {
	return []Arg{{Name: "entryType", Type: StringArg}, {Name: "entry", Type: EntryArg}}
}

func (a *ActionCommit) Do(h *Holochain) (response interface{}, err error) {
	var d *EntryDef
	var entryHash Hash
	//	var header *Header
	d, _, entryHash, err = h.doCommit(a, nil)
	if err != nil {
		return
	}
	if d.DataFormat == DataFormatLinks {
		// if this is a Link entry we have to send the DHT Link message
		var le LinksEntry
		entryStr := a.entry.Content().(string)
		err = json.Unmarshal([]byte(entryStr), &le)
		if err != nil {
			return
		}

		bases := make(map[string]bool)
		for _, l := range le.Links {
			_, exists := bases[l.Base]
			if !exists {
				b, _ := NewHash(l.Base)
				h.dht.Send(b, LINK_REQUEST, LinkReq{Base: b, Links: entryHash})
				//TODO errors from the send??
				bases[l.Base] = true
			}
		}
	} else if d.Sharing == Public {
		// otherwise we check to see if it's a public entry and if so send the DHT put message
		_, err = h.dht.Send(entryHash, PUT_REQUEST, PutReq{H: entryHash})
	}
	response = entryHash
	return
}

// sysValidateEntry does system level validation for an entry
// It checks that entry is not nil, and that it conforms to the entry schema in the definition
// and if it's a Links entry that the contents are correctly structured
func sysValidateEntry(h *Holochain, d *EntryDef, entry Entry) (err error) {
	if entry == nil {
		err = errors.New("nil entry invalid")
		return
	}
	// see if there is a schema validator for the entry type and validate it if so
	if d.validator != nil {
		var input interface{}
		if d.DataFormat == DataFormatJSON {
			if err = json.Unmarshal([]byte(entry.Content().(string)), &input); err != nil {
				return
			}
		} else {
			input = entry
		}
		Debugf("Validating %v against schema", input)
		if err = d.validator.Validate(input); err != nil {
			return
		}
	} else if d.DataFormat == DataFormatLinks {
		// Perform base validation on links entries, i.e. that all items exist and are of the right types
		// so first unmarshall the json, and then check that the hashes are real.
		var l struct{ Links []map[string]string }
		err = json.Unmarshal([]byte(entry.Content().(string)), &l)
		if err != nil {
			err = fmt.Errorf("invalid links entry, invalid json: %v", err)
			return
		}
		if len(l.Links) == 0 {
			err = errors.New("invalid links entry: you must specify at least one link")
			return
		}
		for _, link := range l.Links {
			h, ok := link["Base"]
			if !ok {
				err = errors.New("invalid links entry: missing Base")
				return
			}
			if _, err = NewHash(h); err != nil {
				err = fmt.Errorf("invalid links entry: Base %v", err)
				return
			}
			h, ok = link["Link"]
			if !ok {
				err = errors.New("invalid links entry: missing Link")
				return
			}
			if _, err = NewHash(h); err != nil {
				err = fmt.Errorf("invalid links entry: Link %v", err)
				return
			}
			_, ok = link["Tag"]
			if !ok {
				err = errors.New("invalid links entry: missing Tag")
				return
			}
		}

	}
	return
}

func (a *ActionCommit) SysValidation(h *Holochain, d *EntryDef, sources []peer.ID) (err error) {
	err = sysValidateEntry(h, d, a.entry)
	return
}

func (a *ActionCommit) DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error) {
	err = NonDHTAction
	return
}

func (a *ActionCommit) CheckValidationRequest(def *EntryDef) (err error) {
	return
}

//------------------------------------------------------------
// Put

type ActionPut struct {
	entryType string
	entry     Entry
	header    *Header
}

func NewPutAction(entryType string, entry Entry, header *Header) *ActionPut {
	a := ActionPut{entryType: entryType, entry: entry, header: header}
	return &a
}

func (a *ActionPut) Name() string {
	return "put"
}

func (a *ActionPut) Args() []Arg {
	return nil
}

func (a *ActionPut) Do(h *Holochain) (response interface{}, err error) {
	err = NonCallableAction
	return
}

func (a *ActionPut) SysValidation(h *Holochain, d *EntryDef, sources []peer.ID) (err error) {
	err = sysValidateEntry(h, d, a.entry)
	return
}

func (a *ActionPut) DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error) {
	//dht.puts <- *m  TODO add back in queueing
	err = dht.handleChangeReq(msg)
	response = "queued"
	return
}

func (a *ActionPut) CheckValidationRequest(def *EntryDef) (err error) {
	return
}

//------------------------------------------------------------
// Mod

type ActionMod struct {
	entryType string
	entry     Entry
	header    *Header
	replaces  Hash
}

func NewModAction(entryType string, entry Entry, replaces Hash) *ActionMod {
	a := ActionMod{entryType: entryType, entry: entry, replaces: replaces}
	return &a
}

func (a *ActionMod) Entry() Entry {
	return a.entry
}

func (a *ActionMod) EntryType() string {
	return a.entryType
}

func (a *ActionMod) Name() string {
	return "mod"
}

func (a *ActionMod) Args() []Arg {
	return []Arg{{Name: "entryType", Type: StringArg}, {Name: "entry", Type: EntryArg}, {Name: "replaces", Type: HashArg}}
}

func (a *ActionMod) Do(h *Holochain) (response interface{}, err error) {
	var d *EntryDef
	var entryHash Hash
	d, a.header, entryHash, err = h.doCommit(a, &StatusChange{Action: ModAction, Hash: a.replaces})
	if err != nil {
		return
	}
	if d.Sharing == Public {
		// if it's a public entry send the DHT MOD & PUT messages
		// TODO handle errors better!!
		_, err = h.dht.Send(entryHash, PUT_REQUEST, PutReq{H: entryHash})
		_, err = h.dht.Send(a.replaces, MOD_REQUEST, ModReq{H: a.replaces, N: entryHash})
	}
	response = entryHash
	return
}

func (a *ActionMod) SysValidation(h *Holochain, def *EntryDef, sources []peer.ID) (err error) {
	if def.DataFormat == DataFormatLinks {
		err = errors.New("Can't mod Links entry")
		return
	}
	var header *Header
	header, err = h.chain.GetEntryHeader(a.replaces)
	if err != nil {
		return
	}
	if header.Type != a.entryType {
		err = ErrEntryTypeMismatch
		return
	}
	err = sysValidateEntry(h, def, a.entry)
	return
}

func (a *ActionMod) DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error) {
	//dht.puts <- *m  TODO add back in queueing
	err = dht.handleChangeReq(msg)
	response = "queued"
	return
}

func (a *ActionMod) CheckValidationRequest(def *EntryDef) (err error) {
	return
}

//------------------------------------------------------------
// Del

type ActionDel struct {
	entryType string
	entry     DelEntry
}

func NewDelAction(entryType string, entry DelEntry) *ActionDel {
	a := ActionDel{entryType: entryType, entry: entry}
	return &a
}

func (a *ActionDel) Name() string {
	return "del"
}

func (a *ActionDel) Entry() Entry {
	var buf []byte
	buf, err := ByteEncoder(a.entry)
	if err != nil {
		panic(err)
	}
	return &GobEntry{C: string(buf)}
}

func (a *ActionDel) EntryType() string {
	return a.entryType
}

func (a *ActionDel) Args() []Arg {
	return []Arg{{Name: "hash", Type: HashArg}, {Name: "message", Type: StringArg}}
}

func (a *ActionDel) Do(h *Holochain) (response interface{}, err error) {
	var d *EntryDef
	var entryHash Hash
	d, _, entryHash, err = h.doCommit(a, &StatusChange{Action: DelAction, Hash: a.entry.Hash})
	if err != nil {
		return
	}
	if d.Sharing == Public {
		// if it's a public entry send the DHT DEL
		_, err = h.dht.Send(a.entry.Hash, DEL_REQUEST, DelReq{H: a.entry.Hash, By: entryHash})
	}
	response = entryHash

	return
}

func (a *ActionDel) SysValidation(h *Holochain, d *EntryDef, sources []peer.ID) (err error) {
	if d.DataFormat == DataFormatLinks {
		err = errors.New("Can't del Links entry")
		return
	}
	var header *Header
	header, err = h.chain.GetEntryHeader(a.entry.Hash)
	if err != nil {
		return
	}
	if header.Type != a.entryType {
		err = ErrEntryTypeMismatch
		return
	}
	return
}

func (a *ActionDel) DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error) {
	//dht.puts <- *m  TODO add back in queueing
	err = dht.handleChangeReq(msg)
	response = "queued"
	return
}

func (a *ActionDel) CheckValidationRequest(def *EntryDef) (err error) {
	return
}

//------------------------------------------------------------
// Link

type ActionLink struct {
	entryType      string
	links          []Link
	validationBase Hash
}

func NewLinkAction(entryType string, links []Link) *ActionLink {
	a := ActionLink{entryType: entryType, links: links}
	return &a
}

func (a *ActionLink) Name() string {
	return "link"
}

func (a *ActionLink) Args() []Arg {
	return nil
}

func (a *ActionLink) Do(h *Holochain) (response interface{}, err error) {
	err = NonCallableAction
	return
}

func (a *ActionLink) SysValidation(h *Holochain, d *EntryDef, sources []peer.ID) (err error) {
	//@TODO what sys level links validation?  That they are all valid hash format for the DNA?
	return
}

func (a *ActionLink) DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error) {
	base := msg.Body.(LinkReq).Base
	err = dht.exists(base, StatusLive)
	if err == nil {
		//h.dht.puts <- *m  TODO add back in queueing
		err = dht.handleChangeReq(msg)

		response = "queued"
	} else {
		dht.dlog.Logf("DHTReceiver key %v doesn't exist, ignoring", base)
	}
	return
}

func (a *ActionLink) CheckValidationRequest(def *EntryDef) (err error) {
	if def.DataFormat != DataFormatLinks {
		err = errors.New("hash not of a linking entry")
	}
	return
}

//------------------------------------------------------------
// GetLink

type ActionGetLink struct {
	linkQuery *LinkQuery
	options   *GetLinkOptions
}

func NewGetLinkAction(linkQuery *LinkQuery, options *GetLinkOptions) *ActionGetLink {
	a := ActionGetLink{linkQuery: linkQuery, options: options}
	return &a
}

func (a *ActionGetLink) Name() string {
	return "getLink"
}

func (a *ActionGetLink) Args() []Arg {
	return []Arg{{Name: "base", Type: HashArg}, {Name: "tag", Type: StringArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(GetLinkOptions{}), Optional: true}}
}

func (a *ActionGetLink) Do(h *Holochain) (response interface{}, err error) {
	var r interface{}
	r, err = h.dht.Send(a.linkQuery.Base, GETLINK_REQUEST, *a.linkQuery)

	if err == nil {
		switch t := r.(type) {
		case *LinkQueryResp:
			response = t
			if a.options.Load {
				for i := range t.Links {
					var hash Hash
					hash, err = NewHash(t.Links[i].H)
					if err != nil {
						return
					}
					req := GetReq{H: hash, StatusMask: StatusDefault}
					rsp, err := NewGetAction(req, &GetOptions{StatusMask: StatusDefault}).Do(h)
					if err == nil {
						t.Links[i].E = rsp.(GetResp).Entry.(Entry).Content().(string)
					}
					//TODO better error handling here, i.e break out of the loop and return if error?
				}
			}
		default:
			err = fmt.Errorf("unexpected response type from SendGetLink: %T", t)
		}
	}
	return
}

func (a *ActionGetLink) SysValidation(h *Holochain, d *EntryDef, sources []peer.ID) (err error) {
	//@TODO what sys level getlinks validation?  That they are all valid hash format for the DNA?
	return
}

func (a *ActionGetLink) DHTReqHandler(dht *DHT, msg *Message) (response interface{}, err error) {
	lq := msg.Body.(LinkQuery)
	var r LinkQueryResp
	r.Links, err = dht.getLink(lq.Base, lq.T, lq.StatusMask)
	response = &r

	return
}
