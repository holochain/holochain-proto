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
	SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error)
	Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error)
	CheckValidationRequest(def *EntryDef) (err error)
	Args() []Arg
	EntryType() string
	Entry() Entry
	SetHeader(header *Header)
}

// ValidatingAction provides an abstraction for grouping all the actions that participate in validation loop
type ValidatingAction interface {
	Name() string
	Do(h *Holochain) (response interface{}, err error)
	SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error)
	Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error)
	CheckValidationRequest(def *EntryDef) (err error)
	Args() []Arg
}

var NonDHTAction error = errors.New("Not a DHT action")
var NonCallableAction error = errors.New("Not a callable action")
var ErrNotValidForDNAType error = errors.New("Invalid action for DNA type")
var ErrNotValidForAgentType error = errors.New("Invalid action for Agent type")
var ErrNotValidForKeyType error = errors.New("Invalid action for Key type")
var ErrNilEntryInvalid error = errors.New("nil entry invalid")

func prepareSources(sources []peer.ID) (srcs []string) {
	srcs = make([]string, 0)
	for _, s := range sources {
		srcs = append(srcs, peer.IDB58Encode(s))
	}
	return
}

// ValidateAction runs the different phases of validating an action
func (h *Holochain) ValidateAction(a ValidatingAction, entryType string, pkg *Package, sources []peer.ID) (def *EntryDef, err error) {

	defer func() {
		if err != nil {
			h.dht.dlog.Logf("%T Validation failed with: %v", a, err)
		}
	}()

	var z *Zome
	z, def, err = h.GetEntryDef(entryType)
	if err != nil {
		return
	}

	// run the action's system level validations
	err = a.SysValidation(h, def, pkg, sources)
	if err != nil {
		h.Debugf("Sys ValidateAction(%T) err:%v\n", a, err)
		return
	}
	if !def.IsSysEntry() {

		// validation actions for application defined entry types
		var vpkg *ValidationPackage
		vpkg, err = MakeValidationPackage(h, pkg)
		if err != nil {
			return
		}

		// run the action's app level validations
		var n Ribosome
		n, err = z.MakeRibosome(h)
		if err != nil {
			return
		}

		err = n.ValidateAction(a, def, vpkg, prepareSources(sources))
		if err != nil {
			h.Debugf("Ribosome ValidateAction(%T) err:%v\n", a, err)
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
			var pk []byte
			pk, err = ic.MarshalPublicKey(h.agent.PubKey())
			if err != nil {
				return
			}
			resp.Entry.C = pk
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
		err = ErrNotValidForDNAType
		return
	case KeyEntryType:
		// if key entry there no extra info to return in the package so do nothing
	case AgentEntryType:
		// if agent, the package to return is the entry-type chain
		// so that sys validation can confirm this agent entry in the chain
		req := PackagingReq{PkgReqChain: int64(PkgReqChainOptFull), PkgReqEntryTypes: []string{AgentEntryType}}
		resp.Package, err = MakePackage(h, req)
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
			h.Debugf("Ribosome GetValidationPackage(%T) err:%v\n", a, err)
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
		a = &ActionGetLinks{}
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
	entryType string
	entry     Entry
}

func NewMakeHashAction(entry Entry) *ActionMakeHash {
	a := ActionMakeHash{entry: entry}
	return &a
}

func (a *ActionMakeHash) Name() string {
	return "makeHash"
}

func (a *ActionMakeHash) Args() []Arg {
	return []Arg{{Name: "entryType", Type: StringArg}, {Name: "entry", Type: EntryArg}}
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
	return []Arg{{Name: "doc", Type: StringArg}}
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
	msg := h.node.NewMessage(APP_MESSAGE, a.msg)
	if a.options != nil && a.options.Callback != nil {
		err = h.SendAsync(ActionProtocol, a.to, msg, a.options.Callback, timeout)
	} else {

		r, err = h.Send(h.node.ctx, ActionProtocol, a.to, msg, timeout)
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
			if hash.String() == a.req.H.String() {
				err = errors.New("FollowHash loop detected")
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

func (a *ActionGet) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
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

	// always get the entry type despite what the mas says because we need it for the switch below.
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
	var added bool

	// retry loop incase someone sneaks a new commit in between prepareHeader and addEntry
	for !added {
		h.chain.lk.RLock()
		count := len(h.chain.Headers)
		l, hash, header, err = h.chain.prepareHeader(time.Now(), entryType, entry, h.agent.PrivKey(), change)
		h.chain.lk.RUnlock()
		if err != nil {
			return
		}

		a.SetHeader(header)
		d, err = h.ValidateAction(a, entryType, nil, []peer.ID{h.nodeID})
		if err != nil {
			return
		}

		h.chain.lk.Lock()
		if count == len(h.chain.Headers) {
			err = h.chain.addEntry(l, hash, header, entry)
			if err == nil {
				added = true
			}
		}
		h.chain.lk.Unlock()
		if err != nil {
			return
		}
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

func (a *ActionCommit) SetHeader(header *Header) {
	a.header = header
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

// sysValidateEntry does system level validation for adding an entry (put or commit)
// It checks that entry is not nil, and that it conforms to the entry schema in the definition
// if it's a Links entry that the contents are correctly structured
// if it's a new agent entry, that identity matches the defined identity structure
// if it's a key that the structure is actually a public key
func sysValidateEntry(h *Holochain, def *EntryDef, entry Entry, pkg *Package) (err error) {
	switch def.Name {
	case DNAEntryType:
		err = ErrNotValidForDNAType
		return
	case KeyEntryType:
		pk, ok := entry.Content().([]byte)
		if !ok || len(pk) != 36 {
			err = ValidationFailedErr
			return
		} else {
			_, err = ic.UnmarshalPublicKey(pk)
			if err != nil {
				err = ValidationFailedErr
				return err
			}
		}
	case AgentEntryType:
		ae, ok := entry.Content().(AgentEntry)
		if !ok {
			err = ValidationFailedErr
			return
		}

		// check that the public key is unmarshalable
		_, err = ic.UnmarshalPublicKey(ae.PublicKey)
		if err != nil {
			err = ValidationFailedErr
			return err
		}

		// if there's a revocation, confirm that has a reasonable format
		if ae.Revocation != nil {
			revocation := &SelfRevocation{}
			err := revocation.Unmarshal(ae.Revocation)
			if err != nil {
				err = ValidationFailedErr
				return err
			}
		}

		// TODO check anything in the package
	}

	if entry == nil {
		err = ErrNilEntryInvalid
		return
	}
	// see if there is a schema validator for the entry type and validate it if so
	if def.validator != nil {
		var input interface{}
		if def.DataFormat == DataFormatJSON {
			if err = json.Unmarshal([]byte(entry.Content().(string)), &input); err != nil {
				return
			}
		} else {
			input = entry
		}
		h.Debugf("Validating %v against schema", input)
		if err = def.validator.Validate(input); err != nil {
			return
		}
	} else if def.DataFormat == DataFormatLinks {
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

func (a *ActionCommit) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	err = sysValidateEntry(h, def, a.entry, pkg)
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

func (a *ActionPut) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	err = sysValidateEntry(h, def, a.entry, pkg)
	return
}

func RunValidationPhase(h *Holochain, source peer.ID, msgType MsgType, query Hash, handler func(resp ValidateResponse) error) (err error) {
	var r interface{}
	msg := h.node.NewMessage(msgType, ValidateQuery{H: query})
	r, err = h.Send(h.node.ctx, ValidateProtocol, source, msg, 0)
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

func (a *ActionMod) SetHeader(header *Header) {
	a.header = header
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

func (a *ActionMod) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	switch def.Name {
	case DNAEntryType:
		err = ErrNotValidForDNAType
		return
	case KeyEntryType:
	case AgentEntryType:
	}

	if def.DataFormat == DataFormatLinks {
		err = errors.New("Can't mod Links entry")
		return
	}

	if a.entry == nil {
		err = ErrNilEntryInvalid
		return
	}
	if a.header == nil {
		err = errors.New("mod: missing header")
		return
	}
	if a.replaces.String() == a.header.EntryLink.String() {
		err = errors.New("mod: replaces must be different from original hash")
		return
	}
	// no need to check for virtual entries on the chain because they aren't there
	// currently the only virtual entry is the node id
	/*
		if !def.IsVirtualEntry() {
			var header *Header
			header, err = h.chain.GetEntryHeader(a.replaces)
			if err != nil {
				return
			}
			if header.Type != a.entryType {
				err = ErrEntryTypeMismatch
				return
			}
		}*/
	err = sysValidateEntry(h, def, a.entry, pkg)
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

		//TODO: synchronize this, what happens if two new agent request come in back to back?
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
	header    *Header
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

func (a *ActionDel) SetHeader(header *Header) {
	a.header = header
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

func (a *ActionDel) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	switch def.Name {
	case DNAEntryType:
		err = ErrNotValidForDNAType
		return
	case KeyEntryType:
		err = ErrNotValidForKeyType
		return
	case AgentEntryType:
		err = ErrNotValidForAgentType
		return
	}

	if def.DataFormat == DataFormatLinks {
		err = errors.New("Can't del Links entry")
		return
	}

	// we don't have to check to see if the entry type is virtual here because currently
	// the only virtual entry type is the KeyEntryType, for which Del isn't even valid
	// and will have gotten caught in the switch statement above so no use wasting CPU cycles.
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

func (a *ActionLink) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	if def.DataFormat != DataFormatLinks {
		err = errors.New("action only valid for links entry type")
	}
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
// GetLinks

type ActionGetLinks struct {
	linkQuery *LinkQuery
	options   *GetLinksOptions
}

func NewGetLinksAction(linkQuery *LinkQuery, options *GetLinksOptions) *ActionGetLinks {
	a := ActionGetLinks{linkQuery: linkQuery, options: options}
	return &a
}

func (a *ActionGetLinks) Name() string {
	return "getLinks"
}

func (a *ActionGetLinks) Args() []Arg {
	return []Arg{{Name: "base", Type: HashArg}, {Name: "tag", Type: StringArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(GetLinksOptions{}), Optional: true}}
}

func (a *ActionGetLinks) Do(h *Holochain) (response interface{}, err error) {
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
					opts := GetOptions{GetMask: GetMaskEntryType + GetMaskEntry, StatusMask: StatusDefault}
					req := GetReq{H: hash, StatusMask: StatusDefault, GetMask: opts.GetMask}
					var rsp interface{}
					rsp, err = NewGetAction(req, &opts).Do(h)
					if err == nil {
						// TODO: bleah, really this should be another of those
						// case statements that choses the encoding baste on
						// entry type, time for a refactor!
						entry := rsp.(GetResp).Entry
						switch content := entry.Content().(type) {
						case string:
							t.Links[i].E = content
						case []byte:
							var j []byte
							j, err = json.Marshal(content)
							if err != nil {
								return
							}
							t.Links[i].E = string(j)
						case AgentEntry:
							var j []byte
							j, err = json.Marshal(content)
							if err != nil {
								return
							}
							t.Links[i].E = string(j)
						default:
							err = fmt.Errorf("bad type in entry content: %T:%v", content, content)
						}
						t.Links[i].EntryType = rsp.(GetResp).EntryType
					}
					//TODO better error handling here, i.e break out of the loop and return if error?
				}
			}
		default:
			err = fmt.Errorf("unexpected response type from SendGetLinks: %T", t)
		}
	}
	return
}

func (a *ActionGetLinks) SysValidation(h *Holochain, d *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	//@TODO what sys level getlinks validation?  That they are all valid hash format for the DNA?
	return
}

func (a *ActionGetLinks) Receive(dht *DHT, msg *Message, retries int) (response interface{}, err error) {
	lq := msg.Body.(LinkQuery)
	var r LinkQueryResp
	r.Links, err = dht.getLinks(lq.Base, lq.T, lq.StatusMask)
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
