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
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	ic "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
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
