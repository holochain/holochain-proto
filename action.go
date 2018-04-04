// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/holochain/holochain-proto/hash"
	b58 "github.com/jbenet/go-base58"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
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

// Arg holds the definition of an API function argument
type Arg struct {
	Name     string
	Type     ArgType
	Optional bool
	MapType  reflect.Type
	value    interface{}
}

// APIFunction abstracts the argument structure and the calling of an api function
type APIFunction interface {
	Name() string
	Args() []Arg
	Call(h *Holochain) (response interface{}, err error)
}

// Action provides an abstraction for handling node interaction
type Action interface {
	Name() string
	Receive(dht *DHT, msg *Message) (response interface{}, err error)
}

// CommittingAction provides an abstraction for grouping actions which carry Entry data
type CommittingAction interface {
	Name() string
	SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error)
	Receive(dht *DHT, msg *Message) (response interface{}, err error)
	CheckValidationRequest(def *EntryDef) (err error)
	EntryType() string
	Entry() Entry
	SetHeader(header *Header)
	GetHeader() (header *Header)
	Share(h *Holochain, def *EntryDef) (err error)
}

// ValidatingAction provides an abstraction for grouping all the actions that participate in validation loop
type ValidatingAction interface {
	Name() string
	SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error)
	Receive(dht *DHT, msg *Message) (response interface{}, err error)
	CheckValidationRequest(def *EntryDef) (err error)
}

type ModAgentOptions struct {
	Identity   string
	Revocation string
}

var NonDHTAction error = errors.New("Not a DHT action")
var ErrNotValidForDNAType error = errors.New("Invalid action for DNA type")
var ErrNotValidForAgentType error = errors.New("Invalid action for Agent type")
var ErrNotValidForKeyType error = errors.New("Invalid action for Key type")
var ErrNotValidForHeadersType error = errors.New("Invalid action for Headers type")
var ErrNotValidForDelType error = errors.New("Invalid action for Del type")
var ErrModInvalidForLinks error = errors.New("mod: invalid for Links entry")
var ErrModMissingHeader error = errors.New("mod: missing header")
var ErrModReplacesHashNotDifferent error = errors.New("mod: replaces must be different from original hash")
var ErrEntryDefInvalid = errors.New("Invalid Entry Defintion")

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
			var pk string
			pk, err = h.agent.EncodePubKey()
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
	case HeadersEntryType:
		// if headers entry there no extra info to return in the package so do nothing
	case DelEntryType:
		// if del entry there no extra info to return in the package so do nothing
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
		t = reflect.TypeOf(HoldReq{})
	case GET_REQUEST:
		a = &ActionGet{}
		t = reflect.TypeOf(GetReq{})
	case MOD_REQUEST:
		a = &ActionMod{}
		t = reflect.TypeOf(HoldReq{})
	case DEL_REQUEST:
		a = &ActionDel{}
		t = reflect.TypeOf(HoldReq{})
	case LINK_REQUEST:
		a = &ActionLink{}
		t = reflect.TypeOf(HoldReq{})
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

type APIFnProperty struct {
	prop string
}

func (a *APIFnProperty) Name() string {
	return "property"
}

func (a *APIFnProperty) Args() []Arg {
	return []Arg{{Name: "name", Type: StringArg}}
}

func (a *APIFnProperty) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.GetProperty(a.prop)
	return
}

//------------------------------------------------------------
// Debug

type APIFnDebug struct {
	msg string
}

func (a *APIFnDebug) Name() string {
	return "debug"
}

func (a *APIFnDebug) Args() []Arg {
	return []Arg{{Name: "value", Type: ToStrArg}}
}

func (a *APIFnDebug) Call(h *Holochain) (response interface{}, err error) {
	h.Config.Loggers.App.Log(a.msg)
	return
}

//------------------------------------------------------------
// MakeHash

type APIFnMakeHash struct {
	entryType string
	entry     Entry
}

func (a *APIFnMakeHash) Name() string {
	return "makeHash"
}

func (a *APIFnMakeHash) Args() []Arg {
	return []Arg{{Name: "entryType", Type: StringArg}, {Name: "entry", Type: EntryArg}}
}

func (a *APIFnMakeHash) Call(h *Holochain) (response interface{}, err error) {
	var hash Hash
	hash, err = a.entry.Sum(h.hashSpec)
	if err != nil {
		return
	}
	response = hash
	return
}

//------------------------------------------------------------
// StartBundle

const (
	DefaultBundleTimeout = 5000
)

type APIFnStartBundle struct {
	timeout   int64
	userParam string
}

func NewStartBundleAction(timeout int, userParam string) *APIFnStartBundle {
	a := APIFnStartBundle{timeout: int64(timeout), userParam: userParam}
	if timeout == 0 {
		a.timeout = DefaultBundleTimeout
	}
	return &a
}

func (a *APIFnStartBundle) Name() string {
	return "bundleStart"
}

func (a *APIFnStartBundle) Args() []Arg {
	return []Arg{{Name: "timeout", Type: IntArg}, {Name: "userParam", Type: StringArg}}
}

func (a *APIFnStartBundle) Call(h *Holochain) (response interface{}, err error) {
	err = h.Chain().StartBundle(a.userParam)
	return
}

//------------------------------------------------------------
// CloseBundle

type APIFnCloseBundle struct {
	commit bool
}

func (a *APIFnCloseBundle) Name() string {
	return "bundleClose"
}

func (a *APIFnCloseBundle) Args() []Arg {
	return []Arg{{Name: "commit", Type: BoolArg}}
}

func (a *APIFnCloseBundle) Call(h *Holochain) (response interface{}, err error) {

	bundle := h.Chain().BundleStarted()
	if bundle == nil {
		err = ErrBundleNotStarted
		return
	}

	isCancel := !a.commit
	// if this is a cancel call all the bundleCancel routines
	if isCancel {
		for _, zome := range h.nucleus.dna.Zomes {
			var r Ribosome
			r, _, err = h.MakeRibosome(zome.Name)
			if err != nil {
				continue
			}
			var result string
			result, err = r.BundleCanceled(BundleCancelReasonUserCancel)
			if err != nil {
				Debugf("error in %s.bundleCanceled():%v", zome.Name, err)
				continue
			}
			if result == BundleCancelResponseCommit {
				Debugf("%s.bundleCanceled() overrode cancel", zome.Name)
				err = nil
				return
			}
		}
	}
	err = h.Chain().CloseBundle(a.commit)
	if err == nil {
		// if there wasn't an error closing the bundle share all the commits
		for _, a := range bundle.sharing {
			_, def, err := h.GetEntryDef(a.GetHeader().Type)
			if err != nil {
				h.dht.dlog.Logf("Error getting entry def in close bundle:%v", err)
				err = nil
			} else {
				err = a.Share(h, def)
			}
		}
	}
	return
}

//------------------------------------------------------------
// GetBridges

type APIFnGetBridges struct {
}

func (a *APIFnGetBridges) Name() string {
	return "getBridges"
}

func (a *APIFnGetBridges) Args() []Arg {
	return []Arg{}
}

func (a *APIFnGetBridges) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.GetBridges()
	return
}

//------------------------------------------------------------
// Sign

type APIFnSign struct {
	data []byte
}

func (a *APIFnSign) Name() string {
	return "sign"
}

func (a *APIFnSign) Args() []Arg {
	return []Arg{{Name: "data", Type: StringArg}}
}

func (a *APIFnSign) Call(h *Holochain) (response interface{}, err error) {
	var sig Signature
	sig, err = h.Sign(a.data)
	if err != nil {
		return
	}
	response = sig.B58String()
	return
}

//------------------------------------------------------------
// VerifySignature
type APIFnVerifySignature struct {
	b58signature string
	data         string
	b58pubKey    string
}

func (a *APIFnVerifySignature) Name() string {
	return "verifySignature"
}

func (a *APIFnVerifySignature) Args() []Arg {
	return []Arg{{Name: "signature", Type: StringArg}, {Name: "data", Type: StringArg}, {Name: "pubKey", Type: StringArg}}
}

func (a *APIFnVerifySignature) Call(h *Holochain) (response interface{}, err error) {
	var b bool
	var pubKey ic.PubKey
	sig := SignatureFromB58String(a.b58signature)

	pubKey, err = DecodePubKey(a.b58pubKey)

	b, err = h.VerifySignature(sig, a.data, pubKey)
	if err != nil {
		return
	}
	response = b
	return
}

//------------------------------------------------------------
// Call

type APIFnCall struct {
	zome     string
	function string
	args     interface{}
}

func (fn *APIFnCall) Name() string {
	return "call"
}

func (fn *APIFnCall) Args() []Arg {
	return []Arg{{Name: "zome", Type: StringArg}, {Name: "function", Type: StringArg}, {Name: "args", Type: ArgsArg}}
}

func (fn *APIFnCall) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.Call(fn.zome, fn.function, fn.args, ZOME_EXPOSURE)
	return
}

//------------------------------------------------------------
// Bridge

type APIFnBridge struct {
	token    string
	url      string
	zome     string
	function string
	args     interface{}
}

func (fn *APIFnBridge) Name() string {
	return "bridge"
}

func (fn *APIFnBridge) Args() []Arg {
	return []Arg{{Name: "app", Type: HashArg}, {Name: "zome", Type: StringArg}, {Name: "function", Type: StringArg}, {Name: "args", Type: ArgsArg}}
}

func (fn *APIFnBridge) Call(h *Holochain) (response interface{}, err error) {
	body := bytes.NewBuffer([]byte(fn.args.(string)))
	var resp *http.Response
	resp, err = http.Post(fmt.Sprintf("%s/bridge/%s/%s/%s", fn.url, fn.token, fn.zome, fn.function), "", body)
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

type APIFnSend struct {
	action ActionSend
}

func (fn *APIFnSend) Name() string {
	return "send"
}

func (fn *APIFnSend) Args() []Arg {
	return []Arg{{Name: "to", Type: HashArg}, {Name: "msg", Type: MapArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(SendOptions{}), Optional: true}}
}

func (fn *APIFnSend) Call(h *Holochain) (response interface{}, err error) {
	var r interface{}
	var timeout time.Duration
	a := &fn.action
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

func (a *ActionSend) Name() string {
	return "send"
}

func (a *ActionSend) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
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

type APIFnQuery struct {
	options *QueryOptions
}

func (a *APIFnQuery) Name() string {
	return "query"
}

func (a *APIFnQuery) Args() []Arg {
	return []Arg{{Name: "options", Type: MapArg, MapType: reflect.TypeOf(QueryOptions{}), Optional: true}}
}

func (a *APIFnQuery) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.Query(a.options)
	return
}

//------------------------------------------------------------
// Get

type APIFnGet struct {
	action ActionGet
}

func (fn *APIFnGet) Name() string {
	return fn.action.Name()
}

func (fn *APIFnGet) Args() []Arg {
	return []Arg{{Name: "hash", Type: HashArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(GetOptions{}), Optional: true}}
}

func callGet(h *Holochain, req GetReq, options *GetOptions) (response interface{}, err error) {
	a := ActionGet{req: req, options: options}
	fn := &APIFnGet{action: a}
	response, err = fn.Call(h)
	return
}

func (fn *APIFnGet) Call(h *Holochain) (response interface{}, err error) {
	a := &fn.action
	if a.options.Local {
		response, err = a.getLocal(h.chain)
		return
	}
	if a.options.Bundle {
		bundle := h.Chain().BundleStarted()
		if bundle == nil {
			err = ErrBundleNotStarted
			return
		}
		response, err = a.getLocal(bundle.chain)
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
			modResp, err := callGet(h, req, a.options)
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

type ActionGet struct {
	req     GetReq
	options *GetOptions
}

func (a *ActionGet) Name() string {
	return "get"
}

func (a *ActionGet) getLocal(chain *Chain) (resp GetResp, err error) {
	var entry Entry
	var entryType string
	entry, entryType, err = chain.GetEntry(a.req.H)
	if err != nil {
		return
	}
	resp = GetResp{Entry: *entry.(*GobEntry)}
	mask := a.options.GetMask
	resp.EntryType = entryType
	if (mask & GetMaskEntry) != 0 {
		resp.Entry = *entry.(*GobEntry)
		resp.EntryType = entryType
	}
	return
}

func (a *ActionGet) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	return
}

func (a *ActionGet) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	var entryData []byte
	//var status int
	req := msg.Body.(GetReq)
	mask := req.GetMask
	if mask == GetMaskDefault {
		mask = GetMaskEntry
	}
	resp := GetResp{}
	// always get the entry type despite what the mas says because we need it for the switch below.
	entryData, resp.EntryType, resp.Sources, _, err = dht.Get(req.H, req.StatusMask, req.GetMask|GetMaskEntryType)
	if err == nil {
		if (mask & GetMaskEntry) != 0 {
			switch resp.EntryType {
			case DNAEntryType:
				// TODO: make this add the requester to the blockedlist rather than panicing, see ticket #421
				err = errors.New("nobody should actually get the DNA!")
				return
			case KeyEntryType:
				resp.Entry = GobEntry{C: string(entryData)}
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
			closest := dht.h.node.betterPeersForHash(&req.H, msg.From, true, CloserPeerCount)
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
func (h *Holochain) doCommit(a CommittingAction, change Hash) (d *EntryDef, err error) {

	entryType := a.EntryType()
	entry := a.Entry()
	var l int
	var hash Hash
	var header *Header
	var added bool

	chain := h.Chain()
	bundle := chain.BundleStarted()
	if bundle != nil {
		chain = bundle.chain
	}

	// retry loop incase someone sneaks a new commit in between prepareHeader and addEntry
	for !added {
		chain.lk.RLock()
		count := len(chain.Headers)
		l, hash, header, err = chain.prepareHeader(time.Now(), entryType, entry, h.agent.PrivKey(), change)
		chain.lk.RUnlock()
		if err != nil {
			return
		}

		a.SetHeader(header)
		d, err = h.ValidateAction(a, entryType, nil, []peer.ID{h.nodeID})
		if err != nil {
			return
		}

		chain.lk.Lock()
		if count == len(chain.Headers) {
			err = chain.addEntry(l, hash, header, entry)
			if err == nil {
				added = true
			}
		}
		chain.lk.Unlock()
		if err != nil {
			return
		}
	}
	return
}

func (h *Holochain) commitAndShare(a CommittingAction, change Hash) (response interface{}, err error) {
	var def *EntryDef
	def, err = h.doCommit(a, change)
	if err != nil {
		return
	}

	bundle := h.Chain().BundleStarted()
	if bundle == nil {
		err = a.Share(h, def)
	} else {
		bundle.sharing = append(bundle.sharing, a)
	}
	if err != nil {
		return
	}
	response = a.GetHeader().EntryLink
	return
}

//------------------------------------------------------------
// Commit

type APIFnCommit struct {
	action ActionCommit
}

func (fn *APIFnCommit) Name() string {
	return fn.action.Name()
}

func (fn *APIFnCommit) Args() []Arg {
	return []Arg{{Name: "entryType", Type: StringArg}, {Name: "entry", Type: EntryArg}}
}

func (fn *APIFnCommit) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.commitAndShare(&fn.action, NullHash())
	return
}

func (fn *APIFnCommit) SetAction(a *ActionCommit) {
	fn.action = *a
}

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

func (a *ActionCommit) SetHeader(header *Header) {
	a.header = header
}

func (a *ActionCommit) GetHeader() (header *Header) {
	return a.header
}

func (a *ActionCommit) Share(h *Holochain, def *EntryDef) (err error) {
	if def.DataFormat == DataFormatLinks {
		// if this is a Link entry we have to send the DHT Link message
		var le LinksEntry
		entryStr := a.entry.Content().(string)
		le, err = LinksEntryFromJSON(entryStr)
		if err != nil {
			return
		}

		bases := make(map[string]bool)
		for _, l := range le.Links {
			_, exists := bases[l.Base]
			if !exists {
				b, _ := NewHash(l.Base)
				h.dht.Change(b, LINK_REQUEST, HoldReq{RelatedHash: b, EntryHash: a.header.EntryLink})
				//TODO errors from the send??
				bases[l.Base] = true
			}
		}
	}
	if def.isSharingPublic() {
		// otherwise we check to see if it's a public entry and if so send the DHT put message
		err = h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.header.EntryLink})
		if err == ErrEmptyRoutingTable {
			// will still have committed locally and can gossip later
			err = nil
		}
	}
	return
}

func isValidPubKey(b58pk string) bool {
	if len(b58pk) != 49 {
		return false
	}
	pk := b58.Decode(b58pk)
	_, err := ic.UnmarshalPublicKey(pk)
	if err != nil {
		return false
	}
	return true
}

const (
	ValidationFailureBadPublicKeyFormat  = "bad public key format"
	ValidationFailureBadRevocationFormat = "bad revocation format"
)

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
		b58pk, ok := entry.Content().(string)
		if !ok || !isValidPubKey(b58pk) {
			err = ValidationFailed(ValidationFailureBadPublicKeyFormat)
			return
		}
	case AgentEntryType:
		j, ok := entry.Content().(string)
		if !ok {
			err = ValidationFailedErr
			return
		}
		ae, _ := AgentEntryFromJSON(j)

		// check that the public key is unmarshalable
		if !isValidPubKey(ae.PublicKey) {
			err = ValidationFailed(ValidationFailureBadPublicKeyFormat)
			return err
		}

		// if there's a revocation, confirm that has a reasonable format
		if ae.Revocation != "" {
			revocation := &SelfRevocation{}
			err := revocation.Unmarshal(ae.Revocation)
			if err != nil {
				err = ValidationFailed(ValidationFailureBadRevocationFormat)
				return err
			}
		}

		// TODO check anything in the package
	case HeadersEntryType:
		// TODO check signatures!
	case DelEntryType:
		// TODO checks according to CRDT configuration?
	}

	if entry == nil {
		err = ValidationFailed(ErrNilEntryInvalid.Error())
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
			err = ValidationFailed(err.Error())
			return
		}
		if def == DelEntryDef {
			// TODO refactor and use in other sys types
			hashValue, ok := input.(map[string]interface{})["Hash"].(string)
			if !ok {
				err = ValidationFailed("expected string!")
				return
			}
			_, err = NewHash(hashValue)
			if err != nil {
				err = ValidationFailed(fmt.Sprintf("Error (%s) when decoding Hash value '%s'", err.Error(), hashValue))
				return
			}
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

func (a *ActionCommit) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
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

func (a *ActionPut) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp
	err = RunValidationPhase(dht.h, msg.From, VALIDATE_PUT_REQUEST, t.EntryHash, func(resp ValidateResponse) error {
		a := NewPutAction(resp.Type, &resp.Entry, &resp.Header)
		_, err := dht.h.ValidateAction(a, a.entryType, &resp.Package, []peer.ID{msg.From})

		var status int
		if err != nil {
			dht.dlog.Logf("Put %v rejected: %v", t.EntryHash, err)
			status = StatusRejected
		} else {
			status = StatusLive
		}
		entry := resp.Entry
		var b []byte
		b, err = entry.Marshal()
		if err == nil {
			err = dht.Put(msg, resp.Type, t.EntryHash, msg.From, b, status)
		}
		if err == nil {
			holdResp, err = dht.MakeHoldResp(msg, status)
		}
		return err
	})

	closest := dht.h.node.betterPeersForHash(&t.EntryHash, msg.From, true, CloserPeerCount)
	if len(closest) > 0 {
		err = nil
		resp := CloserPeersResp{}
		resp.CloserPeers = dht.h.node.peers2PeerInfos(closest)
		response = resp
		return
	} else {
		if holdResp != nil {
			response = *holdResp
		}
	}
	return
}

func (a *ActionPut) CheckValidationRequest(def *EntryDef) (err error) {
	return
}

//------------------------------------------------------------
// Mod

type APIFnMod struct {
	action ActionMod
}

func (fn *APIFnMod) Name() string {
	return fn.action.Name()
}

func (fn *APIFnMod) Args() []Arg {
	return []Arg{{Name: "entryType", Type: StringArg}, {Name: "entry", Type: EntryArg}, {Name: "replaces", Type: HashArg}}
}

func (fn *APIFnMod) Call(h *Holochain) (response interface{}, err error) {
	a := &fn.action
	response, err = h.commitAndShare(a, a.replaces)
	return
}

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

func (a *ActionMod) SetHeader(header *Header) {
	a.header = header
}

func (a *ActionMod) GetHeader() (header *Header) {
	return a.header
}

func (a *ActionMod) Share(h *Holochain, def *EntryDef) (err error) {
	if def.isSharingPublic() {
		// if it's a public entry send the DHT MOD & PUT messages
		// TODO handle errors better!!
		h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.header.EntryLink})
		h.dht.Change(a.replaces, MOD_REQUEST, HoldReq{RelatedHash: a.replaces, EntryHash: a.header.EntryLink})
	}
	return
}

func (a *ActionMod) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	switch def.Name {
	case DNAEntryType:
		err = ErrNotValidForDNAType
		return
	case HeadersEntryType:
		err = ErrNotValidForHeadersType
		return
	case DelEntryType:
		err = ErrNotValidForDelType
		return
	case KeyEntryType:
	case AgentEntryType:
	}

	if def.DataFormat == DataFormatLinks {
		err = ErrModInvalidForLinks
		return
	}

	if a.entry == nil {
		err = ErrNilEntryInvalid
		return
	}
	if a.header == nil {
		err = ErrModMissingHeader
		return
	}
	if a.replaces.String() == a.header.EntryLink.String() {
		err = ErrModReplacesHashNotDifferent
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

func (a *ActionMod) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	//var hashStatus int
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_MOD_REQUEST, t.EntryHash, func(resp ValidateResponse) error {
		a := NewModAction(resp.Type, &resp.Entry, t.RelatedHash)
		a.header = &resp.Header

		//@TODO what comes back from Validate Mod
		_, err = dht.h.ValidateAction(a, resp.Type, &resp.Package, []peer.ID{msg.From})
		if err != nil {
			// how do we record an invalid Mod?
			//@TODO store as REJECTED?
		} else {
			err = dht.Mod(msg, t.RelatedHash, t.EntryHash)
			if err == nil {
				holdResp, err = dht.MakeHoldResp(msg, StatusLive)
			}
		}
		return err
	})
	if holdResp != nil {
		response = *holdResp
	}
	return
}

func (a *ActionMod) CheckValidationRequest(def *EntryDef) (err error) {
	return
}

//------------------------------------------------------------
// ModAgent

type APIFnModAgent struct {
	Identity   AgentIdentity
	Revocation string
}

func (fn *APIFnModAgent) Args() []Arg {
	return []Arg{{Name: "options", Type: MapArg, MapType: reflect.TypeOf(ModAgentOptions{})}}
}

func (fn *APIFnModAgent) Name() string {
	return "udpateAgent"
}
func (fn *APIFnModAgent) Call(h *Holochain) (response interface{}, err error) {
	var ok bool
	var newAgent LibP2PAgent = *h.agent.(*LibP2PAgent)
	if fn.Identity != "" {
		newAgent.identity = fn.Identity
		ok = true
	}

	var revocation *SelfRevocation
	if fn.Revocation != "" {
		err = newAgent.GenKeys(nil)
		if err != nil {
			return
		}
		revocation, err = NewSelfRevocation(h.agent.PrivKey(), newAgent.PrivKey(), []byte(fn.Revocation))
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

			h.dht.Change(oldKey, MOD_REQUEST, HoldReq{RelatedHash: oldKey, EntryHash: newKey})

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

type APIFnDel struct {
	action ActionDel
}

func (fn *APIFnDel) Name() string {
	return fn.action.Name()
}

func (fn *APIFnDel) Args() []Arg {
	return []Arg{{Name: "hash", Type: HashArg}, {Name: "message", Type: StringArg}}
}

func (fn *APIFnDel) Call(h *Holochain) (response interface{}, err error) {
	a := &fn.action
	response, err = h.commitAndShare(a, NullHash())
	return
}

type ActionDel struct {
	entry  DelEntry
	header *Header
}

func NewDelAction(entry DelEntry) *ActionDel {
	a := ActionDel{entry: entry}
	return &a
}

func (a *ActionDel) Name() string {
	return "del"
}

func (a *ActionDel) Entry() Entry {
	j, err := a.entry.ToJSON()
	if err != nil {
		panic(err)
	}
	return &GobEntry{C: j}
}

func (a *ActionDel) EntryType() string {
	return DelEntryType
}

func (a *ActionDel) SetHeader(header *Header) {
	a.header = header
}

func (a *ActionDel) GetHeader() (header *Header) {
	return a.header
}

func (a *ActionDel) Share(h *Holochain, def *EntryDef) (err error) {
	if def.isSharingPublic() {
		// if it's a public entry send the DHT DEL & PUT messages
		h.dht.Change(a.header.EntryLink, PUT_REQUEST, HoldReq{EntryHash: a.header.EntryLink})
		h.dht.Change(a.entry.Hash, DEL_REQUEST, HoldReq{RelatedHash: a.entry.Hash, EntryHash: a.header.EntryLink})
	}
	return
}

func (a *ActionDel) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	if def != DelEntryDef {
		err = ErrEntryDefInvalid
		return
	}
	return
}

func (a *ActionDel) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_DEL_REQUEST, t.EntryHash, func(resp ValidateResponse) error {

		var delEntry DelEntry
		delEntry, err = DelEntryFromJSON(resp.Entry.Content().(string))

		a := NewDelAction(delEntry)
		//@TODO what comes back from Validate Del
		_, err = dht.h.ValidateAction(a, resp.Type, &resp.Package, []peer.ID{msg.From})
		if err != nil {
			// how do we record an invalid DEL?
			//@TODO store as REJECTED
		} else {
			err = dht.Del(msg, delEntry.Hash)
			if err == nil {
				holdResp, err = dht.MakeHoldResp(msg, StatusLive)
			}
		}
		return err
	})
	if holdResp != nil {
		response = *holdResp
	}
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

func (a *ActionLink) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	if def.DataFormat != DataFormatLinks {
		err = errors.New("action only valid for links entry type")
	}
	//@TODO what sys level links validation?  That they are all valid hash format for the DNA?
	return
}

func (a *ActionLink) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(HoldReq)
	var holdResp *HoldResp

	err = RunValidationPhase(dht.h, msg.From, VALIDATE_LINK_REQUEST, t.EntryHash, func(resp ValidateResponse) error {
		var le LinksEntry
		le, err = LinksEntryFromJSON(resp.Entry.Content().(string))
		if err != nil {
			return err
		}

		a := NewLinkAction(resp.Type, le.Links)
		a.validationBase = t.RelatedHash
		_, err = dht.h.ValidateAction(a, a.entryType, &resp.Package, []peer.ID{msg.From})
		//@TODO this is "one bad apple spoils the lot" because the app
		// has no way to tell us not to link certain of the links.
		// we need to extend the return value of the app to be able to
		// have it reject a subset of the links.
		if err != nil {
			// how do we record an invalid linking?
			//@TODO store as REJECTED
		} else {
			base := t.RelatedHash.String()
			for _, l := range le.Links {
				if base == l.Base {
					if l.LinkAction == DelLinkAction {
						err = dht.DelLink(msg, base, l.Link, l.Tag)
					} else {
						err = dht.PutLink(msg, base, l.Link, l.Tag)
					}
				}
			}
			if err == nil {
				holdResp, err = dht.MakeHoldResp(msg, StatusLive)
			}
		}
		return err
	})

	if holdResp != nil {
		response = *holdResp
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
// GetLinks

type APIFnGetLinks struct {
	action ActionGetLinks
}

func (fn *APIFnGetLinks) Name() string {
	return fn.action.Name()
}

func (fn *APIFnGetLinks) Args() []Arg {
	return []Arg{{Name: "base", Type: HashArg}, {Name: "tag", Type: StringArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(GetLinksOptions{}), Optional: true}}
}

func (fn *APIFnGetLinks) Call(h *Holochain) (response interface{}, err error) {
	var r interface{}
	a := &fn.action
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
					rsp, err = callGet(h, req, &opts)
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

func (a *ActionGetLinks) SysValidation(h *Holochain, d *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	//@TODO what sys level getlinks validation?  That they are all valid hash format for the DNA?
	return
}

func (a *ActionGetLinks) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	lq := msg.Body.(LinkQuery)
	var r LinkQueryResp
	r.Links, err = dht.GetLinks(lq.Base, lq.T, lq.StatusMask)
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

var prefix string = "List add request rejected on warrant failure"

func (a *ActionListAdd) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
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
