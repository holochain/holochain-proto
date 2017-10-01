// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	b58 "github.com/jbenet/go-base58"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	"io/ioutil"
	"net/http"
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

const (
	DHTChangeOK = iota
	DHTChangeUnknownHashQueuedForRetry
)

type Arg struct {
	Name     string
	Type     ArgType
	Optional bool
	MapType  reflect.Type
	value    interface{}
}

type ModAgentOptions struct {
	Identity   string
	Revocation string
}

// Action provides an abstraction for grouping all the aspects of a nucleus function, i.e.
// the initiating actions, receiving them, validation, ribosome generation etc
type Action interface {
	Name() string
	Do(h *Holochain) (response interface{}, err error)
	Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error)
	Args() []Arg
}

// CommittingAction provides an abstraction for grouping actions which carry Entry data
type CommittingAction interface {
	Name() string
	Do(h *Holochain) (response interface{}, err error)
	SysValidation(h *Holochain, d *EntryDef, sources []peer.ID) (err error)
	Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error)
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
	Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error)
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
		//		validate the public key?
	case AgentEntryType:
		//		validate the Agent Entry?
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
		var n Ribosome
		n, err = z.MakeRibosome(h)
		if err != nil {
			return
		}

		err = n.ValidateAction(a, d, vpkg, prepareSources(sources))
		if err != nil {
			Debugf("Ribosome ValidateAction(%T) err:%v\n", a, err)
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
		if hash.String() == h.nodeIDStr {
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
		//		resp.Entry = TODO agent block goes here
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
		var n Ribosome
		n, err = z.MakeRibosome(h)
		if err != nil {
			return
		}

		var req PackagingReq
		req, err = n.ValidatePackagingRequest(a, def)
		if err != nil {
			Debugf("Ribosome GetValidationPackage(%T) err:%v\n", a, err)
		}
		resp.Package, err = MakePackage(h, req)
	}
	return
}

// MakeActionFromMessage generates an action from an action protocol messsage
func MakeActionFromMessage(msg *Message) (a Action, err error) {
	var t reflect.Type
	switch msg.Type {
	case APP_MESSAGE:
		a = &ActionSend{}
		t = reflect.TypeOf(AppMsg{})
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
	case LISTADD_REQUEST:
		a = &ActionListAdd{}
		t = reflect.TypeOf(ListAddReq{})
	default:
		err = fmt.Errorf("message type %d not in holochain-action protocol", int(msg.Type))
	}
	if err == nil && reflect.TypeOf(msg.Body) != t {
		err = fmt.Errorf("Unexpected request body type '%T' in %s request, expecting %v", msg.Body, a.Name(), t)
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
	h.Config.Loggers.App.Log(a.msg)
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
// GetBridges

type ActionGetBridges struct {
}

func NewGetBridgesAction(doc []byte) *ActionGetBridges {
	a := ActionGetBridges{}
	return &a
}

func (a *ActionGetBridges) Name() string {
	return "getBridges"
}

func (a *ActionGetBridges) Args() []Arg {
	return []Arg{}
}

func (a *ActionGetBridges) Do(h *Holochain) (response interface{}, err error) {
	response, err = h.GetBridges()
	return
}

//------------------------------------------------------------
// Sign

type ActionSign struct {
	doc []byte
}

func NewSignAction(doc []byte) *ActionSign {
	a := ActionSign{doc: doc}
	return &a
}

func (a *ActionSign) Name() string {
	return "sign"
}

func (a *ActionSign) Args() []Arg {
	return []Arg{{Name: "doc", Type: EntryArg}}
}

func (a *ActionSign) Do(h *Holochain) (response interface{}, err error) {
	var b []byte
	b, err = h.Sign(a.doc)
	if err != nil {
		return
	}
	response = b
	return
}

//------------------------------------------------------------
// VerifySignature
type ActionVerifySignature struct {
	signature string
	data      string
	pubKey    string
}

func NewVerifySignatureAction(signature string, data string, pubKey string) *ActionVerifySignature {
	a := ActionVerifySignature{signature: signature, data: data, pubKey: pubKey}
	return &a
}

func (a *ActionVerifySignature) Name() string {
	return "verifySignature"
}

func (a *ActionVerifySignature) Args() []Arg {
	return []Arg{{Name: "signature", Type: StringArg}, {Name: "data", Type: StringArg}, {Name: "pubKey", Type: StringArg}}
}

func (a *ActionVerifySignature) Do(h *Holochain) (response bool, err error) {
	var b bool
	var pubKeyIC ic.PubKey
	var sig []byte
	sig = b58.Decode(a.signature)
	var pubKeyBytes []byte
	pubKeyBytes = b58.Decode(a.pubKey)
	pubKeyIC, err = ic.UnmarshalPublicKey(pubKeyBytes)
	if err != nil {
		return
	}

	b, err = h.VerifySignature(sig, a.data, pubKeyIC)
	if err != nil {
		return
	}
	response = b
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
// Bridge

type ActionBridge struct {
	token    string
	url      string
	zome     string
	function string
	args     interface{}
}

func NewBridgeAction(zome string, function string, args interface{}) *ActionBridge {
	a := ActionBridge{zome: zome, function: function, args: args}
	return &a
}

func (a *ActionBridge) Name() string {
	return "call"
}

func (a *ActionBridge) Args() []Arg {
	return []Arg{{Name: "app", Type: HashArg}, {Name: "zome", Type: StringArg}, {Name: "function", Type: StringArg}, {Name: "args", Type: ArgsArg}}
}

func (a *ActionBridge) Do(h *Holochain) (response interface{}, err error) {
	body := bytes.NewBuffer([]byte(a.args.(string)))
	var resp *http.Response
	resp, err = http.Post(fmt.Sprintf("%s/bridge/%s/%s/%s", a.url, a.token, a.zome, a.function), "", body)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var b []byte
	b, err = ioutil.ReadAll(resp.Body)
	response = string(b)
	return
}

//------------------------------------------------------------
// Send

type Callback struct {
	Function string
	ID       string
	zomeType string
}

type SendOptions struct {
	Callback *Callback
	Timeout  int
}

type ActionSend struct {
	to      peer.ID
	msg     AppMsg
	options *SendOptions
}

func NewSendAction(to peer.ID, msg AppMsg) *ActionSend {
	a := ActionSend{to: to, msg: msg}
	return &a
}

func (a *ActionSend) Name() string {
	return "send"
}

func (a *ActionSend) Args() []Arg {
	return []Arg{{Name: "to", Type: HashArg}, {Name: "msg", Type: MapArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(SendOptions{}), Optional: true}}
}

func (a *ActionSend) Do(h *Holochain) (response interface{}, err error) {
	var r interface{}
	var timeout time.Duration
	if a.options != nil {
		timeout = time.Duration(a.options.Timeout) * time.Millisecond
	}
	if a.options != nil && a.options.Callback != nil {
		err = h.SendAsync(ActionProtocol, a.to, APP_MESSAGE, a.msg, a.options.Callback, timeout)
	} else {
		r, err = h.Send(h.node.ctx, ActionProtocol, a.to, APP_MESSAGE, a.msg, timeout)
		if err == nil {
			response = r.(AppMsg).Body
		}
	}
	return
}

func (a *ActionSend) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
	t := msg.Body.(AppMsg)
	var r Ribosome
	r, _, err = dht.h.MakeRibosome(t.ZomeType)
	if err != nil {
		return
	}
	rsp := AppMsg{ZomeType: t.ZomeType}
	rsp.Body, err = r.Receive(peer.IDB58Encode(msg.From), t.Body)
	if err == nil {
		response = rsp
	}
	return
}

//------------------------------------------------------------
// Query

type ActionQuery struct {
	options *QueryOptions
}

func NewQueryAction(options *QueryOptions) *ActionQuery {
	a := ActionQuery{options: options}
	return &a
}

func (a *ActionQuery) Name() string {
	return "query"
}

func (a *ActionQuery) Args() []Arg {
	return []Arg{{Name: "options", Type: MapArg, MapType: reflect.TypeOf(QueryOptions{}), Optional: true}}
}

func (a *ActionQuery) Do(h *Holochain) (response interface{}, err error) {
	response, err = h.Query(a.options)
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
	if a.options.Local {
		var entry Entry
		var entryType string
		entry, entryType, err = h.chain.GetEntry(a.req.H)
		if err != nil {
			return
		}
		resp := GetResp{Entry: *entry.(*GobEntry)}
		mask := a.options.GetMask
		if (mask & GetMaskEntryType) != 0 {
			resp.EntryType = entryType
		}
		if (mask & GetMaskEntry) != 0 {
			resp.Entry = *entry.(*GobEntry)
		}

		response = resp
		return
	}
	rsp, err := h.dht.Query(a.req.H, GET_REQUEST, a.req)
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

func (a *ActionGet) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
	var entryData []byte
	//var status int
	req := msg.Body.(GetReq)
	mask := req.GetMask
	if mask == GetMaskDefault {
		mask = GetMaskEntry
	}
	resp := GetResp{}
	var entryType string
	entryData, entryType, resp.Sources, _, err = dht.get(req.H, req.StatusMask, req.GetMask|GetMaskEntryType)
	if (mask & GetMaskEntryType) != 0 {
		resp.EntryType = entryType
	}

	if err == nil {
		if (mask & GetMaskEntry) != 0 {
			switch entryType {
			case DNAEntryType:
				// TODO: make this add the requester to the blockedlist rather than panicing, see ticket #421
				err = errors.New("nobody should actually get the DNA!")
				return
			case KeyEntryType:
				resp.Entry = GobEntry{C: entryData}
			default:
				var e GobEntry
				err = e.Unmarshal(entryData)
				if err != nil {
					return
				}
				resp.Entry = e
			}
		}
	} else {
		if err == ErrHashModified {
			resp.FollowHash = string(entryData)
		} else if err == ErrHashNotFound {
			closest := dht.h.node.betterPeersForHash(&req.H, msg.From, CloserPeerCount)
			if len(closest) > 0 {
				err = nil
				resp := CloserPeersResp{}
				resp.CloserPeers = dht.h.node.peers2PeerInfos(closest)
				response = resp
				return
			}
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

	l, hash, header, err = h.chain.PrepareHeader(time.Now(), entryType, entry, h.agent.PrivKey(), change)
	if err != nil {
		return
	}
	//TODO	a.header = header
	d, err = h.ValidateAction(a, entryType, nil, []peer.ID{h.nodeID})
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
				h.dht.Change(b, LINK_REQUEST, LinkReq{Base: b, Links: entryHash})
				//TODO errors from the send??
				bases[l.Base] = true
			}
		}
	} else if d.Sharing == Public {
		// otherwise we check to see if it's a public entry and if so send the DHT put message
		err = h.dht.Change(entryHash, PUT_REQUEST, PutReq{H: entryHash})
		if err == ErrEmptyRoutingTable {
			// will still have committed locally and can gossip later
			err = nil
		}
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

func (a *ActionCommit) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
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

func RunValidationPhase(h *Holochain, source peer.ID, msgType MsgType, query Hash, handler func(resp ValidateResponse) error) (err error) {
	var r interface{}
	r, err = h.Send(h.node.ctx, ValidateProtocol, source, msgType, ValidateQuery{H: query}, 0)
	if err != nil {
		return
	}
	switch resp := r.(type) {
	case ValidateResponse:
		err = handler(resp)
	default:
		err = fmt.Errorf("expected ValidateResponse from validator got %T", r)
	}
	return
}

func (a *ActionPut) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
	t := msg.Body.(PutReq)
	err = RunValidationPhase(dht.h, msg.From, VALIDATE_PUT_REQUEST, t.H, func(resp ValidateResponse) error {
		a := NewPutAction(resp.Type, &resp.Entry, &resp.Header)
		_, err := dht.h.ValidateAction(a, a.entryType, &resp.Package, []peer.ID{msg.From})

		var status int
		if err != nil {
			dht.dlog.Logf("Put %v rejected: %v", t.H, err)
			status = StatusRejected
		} else {
			status = StatusLive
		}
		entry := resp.Entry
		var b []byte
		b, err = entry.Marshal()
		if err == nil {
			err = dht.put(msg, resp.Type, t.H, msg.From, b, status)
		}
		return err
	})

	closest := dht.h.node.betterPeersForHash(&t.H, msg.From, CloserPeerCount)
	if len(closest) > 0 {
		err = nil
		resp := CloserPeersResp{}
		resp.CloserPeers = dht.h.node.peers2PeerInfos(closest)
		response = resp
		return
	} else {
		response = DHTChangeOK
	}
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
		h.dht.Change(entryHash, PUT_REQUEST, PutReq{H: entryHash})
		h.dht.Change(a.replaces, MOD_REQUEST, ModReq{H: a.replaces, N: entryHash})
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

func (a *ActionMod) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
	//var hashStatus int
	t := msg.Body.(ModReq)
	from := msg.From

	response, err = dht.retryIfHashNotFound(t.H, msg, retries)
	if response != nil || err != nil {
		return
	}

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_MOD_REQUEST, t.N, func(resp ValidateResponse) error {
		a := NewModAction(resp.Type, &resp.Entry, t.H)
		a.header = &resp.Header
		//@TODO what comes back from Validate Mod
		_, err = dht.h.ValidateAction(a, resp.Type, &resp.Package, []peer.ID{from})
		if err != nil {
			// how do we record an invalid Mod?
			//@TODO store as REJECTED?
		} else {
			err = dht.mod(msg, t.H, t.N)
		}
		return err
	})
	response = DHTChangeOK
	return
}

func (a *ActionMod) CheckValidationRequest(def *EntryDef) (err error) {
	return
}

//------------------------------------------------------------
// ModAgent

type ActionModAgent struct {
	Identity   AgentIdentity
	Revocation string
}

func NewModAgentAction(identity AgentIdentity) *ActionModAgent {
	a := ActionModAgent{Identity: identity}
	return &a
}

func (a *ActionModAgent) Args() []Arg {
	return []Arg{{Name: "options", Type: MapArg, MapType: reflect.TypeOf(ModAgentOptions{})}}
}

func (a *ActionModAgent) Name() string {
	return "udpateAgent"
}
func (a *ActionModAgent) Do(h *Holochain) (response interface{}, err error) {
	var ok bool
	var newAgent LibP2PAgent = *h.agent.(*LibP2PAgent)
	if a.Identity != "" {
		newAgent.identity = a.Identity
		ok = true
	}

	var revocation *SelfRevocation
	if a.Revocation != "" {
		err = newAgent.GenKeys(nil)
		if err != nil {
			return
		}
		revocation, err = NewSelfRevocation(h.agent.PrivKey(), newAgent.PrivKey(), []byte(a.Revocation))
		if err != nil {
			return
		}
		ok = true
	}
	if !ok {
		err = errors.New("expecting identity and/or revocation option")
	} else {
		h.agent = &newAgent
		// add a new agent entry and update
		var agentHash Hash
		_, agentHash, err = h.AddAgentEntry(revocation)
		if err != nil {
			return
		}
		h.agentTopHash = agentHash

		// if there was a revocation put the new key to the DHT and then reset the node ID data
		// TODO make sure this doesn't introduce race conditions in the DHT between new and old identity #284
		if revocation != nil {
			err = h.dht.putKey(&newAgent)
			if err != nil {
				return
			}

			// send the modification request for the old key
			var oldKey, newKey Hash
			oldPeer := h.nodeID
			oldKey, err = NewHash(h.nodeIDStr)
			if err != nil {
				panic(err)
			}

			h.nodeID, h.nodeIDStr, err = h.agent.NodeID()
			if err != nil {
				return
			}

			newKey, err = NewHash(h.nodeIDStr)
			if err != nil {
				panic(err)
			}

			// close the old node and add the new node
			// TODO currently ignoring the error from node.Close() is this OK?
			h.node.Close()
			h.createNode()

			h.dht.Change(oldKey, MOD_REQUEST, ModReq{H: oldKey, N: newKey})

			warrant, _ := NewSelfRevocationWarrant(revocation)
			var data []byte
			data, err = warrant.Encode()
			if err != nil {
				return
			}

			// TODO, this isn't really a DHT send, but a management send, so the key is bogus.  have to work this out...
			h.dht.Change(oldKey, LISTADD_REQUEST,
				ListAddReq{
					ListType:    BlockedList,
					Peers:       []string{peer.IDB58Encode(oldPeer)},
					WarrantType: SelfRevocationType,
					Warrant:     data,
				})

		}

		response = agentHash
	}
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
		h.dht.Change(a.entry.Hash, DEL_REQUEST, DelReq{H: a.entry.Hash, By: entryHash})
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

func (a *ActionDel) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
	t := msg.Body.(DelReq)
	from := msg.From
	response, err = dht.retryIfHashNotFound(t.H, msg, retries)
	if response != nil || err != nil {
		return
	}

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_DEL_REQUEST, t.By, func(resp ValidateResponse) error {
		var delEntry DelEntry
		err := ByteDecoder([]byte(resp.Entry.Content().(string)), &delEntry)
		if err != nil {
			return err
		}

		a := NewDelAction(resp.Type, delEntry)
		//@TODO what comes back from Validate Del
		_, err = dht.h.ValidateAction(a, resp.Type, &resp.Package, []peer.ID{from})
		if err != nil {
			// how do we record an invalid DEL?
			//@TODO store as REJECTED
		} else {
			err = dht.del(msg, delEntry.Hash)
		}
		return err
	})
	response = DHTChangeOK
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

func (a *ActionLink) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
	t := msg.Body.(LinkReq)
	base := t.Base
	from := msg.From

	response, err = dht.retryIfHashNotFound(base, msg, retries)
	if response != nil || err != nil {
		return
	}

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_LINK_REQUEST, t.Links, func(resp ValidateResponse) error {
		var le LinksEntry

		if err = json.Unmarshal([]byte(resp.Entry.Content().(string)), &le); err != nil {
			return err
		}

		a := NewLinkAction(resp.Type, le.Links)
		a.validationBase = t.Base
		_, err = dht.h.ValidateAction(a, a.entryType, &resp.Package, []peer.ID{from})
		//@TODO this is "one bad apple spoils the lot" because the app
		// has no way to tell us not to link certain of the links.
		// we need to extend the return value of the app to be able to
		// have it reject a subset of the links.
		if err != nil {
			// how do we record an invalid linking?
			//@TODO store as REJECTED
		} else {
			base := t.Base.String()
			for _, l := range le.Links {
				if base == l.Base {
					if l.LinkAction == DelAction {
						err = dht.delLink(msg, base, l.Link, l.Tag)
					} else {
						err = dht.putLink(msg, base, l.Link, l.Tag)
					}
				}
			}
		}
		return err
	})

	response = DHTChangeOK
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
	r, err = h.dht.Query(a.linkQuery.Base, GETLINK_REQUEST, *a.linkQuery)

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
						entry := rsp.(GetResp).Entry
						t.Links[i].E = entry.Content().(string)
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

func (a *ActionGetLink) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
	lq := msg.Body.(LinkQuery)
	var r LinkQueryResp
	r.Links, err = dht.getLink(lq.Base, lq.T, lq.StatusMask)
	response = &r

	return
}

//------------------------------------------------------------
// ListAdd

type ActionListAdd struct {
	list PeerList
}

func NewListAddAction(peerList PeerList) *ActionListAdd {
	a := ActionListAdd{list: peerList}
	return &a
}

func (a *ActionListAdd) Name() string {
	return "put"
}

func (a *ActionListAdd) Args() []Arg {
	return nil
}

func (a *ActionListAdd) Do(h *Holochain) (response interface{}, err error) {
	err = NonCallableAction
	return
}

var prefix string = "List add request rejected on warrant failure"

func (a *ActionListAdd) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
	t := msg.Body.(ListAddReq)
	a.list.Type = PeerListType(t.ListType)
	a.list.Records = make([]PeerRecord, 0)
	var pid peer.ID
	for _, pStr := range t.Peers {
		pid, err = peer.IDB58Decode(pStr)
		if err != nil {
			return
		}
		r := PeerRecord{ID: pid}
		a.list.Records = append(a.list.Records, r)
	}

	// validate the warrant sent with the list add request
	var w Warrant
	w, err = DecodeWarrant(t.WarrantType, t.Warrant)
	if err != nil {
		err = fmt.Errorf("%s: unable to decode warrant (%v)", prefix, err)
		return
	}

	err = w.Verify(dht.h)
	if err != nil {
		err = fmt.Errorf("%s: %v", prefix, err)
		return
	}

	// TODO verify that the warrant, if valid, is sufficient to allow list addition #300

	err = dht.addToList(msg, a.list)
	if err != nil {
		return
	}

	// special case to add blockedlist peers to node cache and delete them from the gossipers list
	if a.list.Type == BlockedList {
		for _, node := range a.list.Records {
			dht.h.node.Block(node.ID)
			dht.DeleteGossiper(node.ID) // ignore error
		}
	}
	response = DHTChangeOK
	return
}

// retryIfHashNotFound checks to see if the hash is found and if not queues the message for retry
func (dht *DHT) retryIfHashNotFound(hash Hash, msg *Message, retries int) (response interface{}, err error) {
	err = dht.exists(hash, StatusDefault)
	if err != nil {
		if err == ErrHashNotFound {
			dht.dlog.Logf("don't yet have %s, trying again later", hash)
			retry := &retry{msg: *msg, retries: retries}
			dht.retryQueue <- retry
			response = DHTChangeUnknownHashQueuedForRetry
			err = nil
		}
	}
	return
}
