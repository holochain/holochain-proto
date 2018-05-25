// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/holochain/holochain-proto/hash"
	b58 "github.com/jbenet/go-base58"
	ic "github.com/libp2p/go-libp2p-crypto"
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
	// returns a GobEntry containing the action's entry in a serialized format
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
	case MigrateEntryType:
		// if migrate entry there no extra info to return in the package so do nothing
		// TODO: later this might not be true, could return whole chain?
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
	case MIGRATE_REQUEST:
		a = &ActionMigrate{}
		t = reflect.TypeOf(HoldReq{})
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
