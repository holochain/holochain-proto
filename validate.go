// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// Chain validation protocol.  This protocol allows DHT nodes to request data so they can
// run validation on the puts and linkings they are asked to perform

package holochain

import (
	"bytes"
	"fmt"
	. "github.com/metacurrency/holochain/hash"
)

// Package holds app specified data needed for validation (wire package)
type Package struct {
	Chain []byte
}

// ValidationPackage holds app specified data needed for validation. This version
// holds the package with any chain data un-marshaled after validation for passing
// into the app for app level validation
type ValidationPackage struct {
	Chain *Chain
}

const (

	// Constants for the key values of the validation request object returned
	// by validateXPkg functions

	// PkgReqChain is the key who value is one of the PkgReqChainOptX masks
	PkgReqChain = "chain"

	// PkgReqEntryTypes is the key who value is an array of entry types to limit
	// the chain to
	PkgReqEntryTypes = "types"

	// Constant mask values for PkgReqChain key of the validation request object

	PkgReqChainOptNone       = 0x00
	PkgReqChainOptHeaders    = 0x01
	PkgReqChainOptEntries    = 0x02
	PkgReqChainOptFull       = 0x03
	PkgReqChainOptNoneStr    = "0"
	PkgReqChainOptHeadersStr = "1"
	PkgReqChainOptEntriesStr = "2"
	PkgReqChainOptFullStr    = "3"
)

// PackagingReq holds a request from an app for data to be included in the validation response
type PackagingReq map[string]interface{}

// ValidateQuery holds the data from a validation query on the Source protocol
type ValidateQuery struct {
	H Hash
}

// ValidateResponse holds the response to committing validates (PUT/MOD/DEL)
type ValidateResponse struct {
	Type    string
	Header  Header
	Entry   GobEntry
	Package Package
}

// MakePackage converts a package request into a package, loading chain data as necessary
// this is the package that gets sent over the wire.  Chain DNA is omitted in this package
// because it can be added at the destination and the chain will still validate.
func MakePackage(h *Holochain, req PackagingReq) (pkg Package, err error) {
	if f, ok := req[PkgReqChain]; ok {
		var b bytes.Buffer
		flags := f.(int64)
		var mflags int64
		if (flags & PkgReqChainOptHeaders) == 0 {
			mflags += ChainMarshalFlagsNoHeaders
		}
		if (flags & PkgReqChainOptEntries) == 0 {
			mflags += ChainMarshalFlagsNoEntries
		}

		var types []string
		if t, ok := req[PkgReqEntryTypes]; ok {
			types = t.([]string)
		}

		privateEntries := h.GetPrivateEntryDefs()
		privateTypeNames := make([]string, len(privateEntries))
		for i, def := range privateEntries {
			privateTypeNames[i] = def.Name
		}

		h.chain.MarshalChain(&b, mflags+ChainMarshalFlagsOmitDNA, types, privateTypeNames)
		pkg.Chain = b.Bytes()
	}
	return
}

// MakeValidationPackage converts a received Package into a ValidationPackage and validates
// any chain data that was included
func MakeValidationPackage(h *Holochain, pkg *Package) (vpkg *ValidationPackage, err error) {
	vp := ValidationPackage{}
	if (pkg != nil) && (pkg.Chain != nil) {
		buf := bytes.NewBuffer(pkg.Chain)
		var flags int64
		flags, vp.Chain, err = UnmarshalChain(h.hashSpec, buf)
		if err != nil {
			return
		}
		if flags&ChainMarshalFlagsNoEntries == 0 {
			// restore the chain's DNA data
			vp.Chain.Entries[0].(*GobEntry).C = h.chain.Entries[0].(*GobEntry).C
		}
		if flags&ChainMarshalFlagsNoHeaders == 0 {
			err = vp.Chain.Validate(flags&ChainMarshalFlagsNoEntries != 0)
			if err != nil {
				return
			}
		}
	}
	vpkg = &vp
	return
}

// ValidateReceiver handles messages on the Validate protocol
func ValidateReceiver(h *Holochain, msg *Message) (response interface{}, err error) {
	var a ValidatingAction
	switch msg.Type {
	case VALIDATE_PUT_REQUEST:
		a = &ActionPut{}
	case VALIDATE_MOD_REQUEST:
		a = &ActionMod{}
	case VALIDATE_DEL_REQUEST:
		a = &ActionDel{}
	case VALIDATE_LINK_REQUEST:
		a = &ActionLink{}
	default:
		err = fmt.Errorf("message type %d not in holochain-validate protocol", int(msg.Type))
	}
	if err == nil {
		h.dht.dlog.Logf("got validate %s request: %v", a.Name(), msg)
		switch t := msg.Body.(type) {
		case ValidateQuery:
			response, err = h.GetValidationResponse(a, t.H)
		default:
			err = fmt.Errorf("expected ValidateQuery got %T", t)
		}
	}
	h.dht.dlog.Logf("validate responding with: %T %v (err=%v)", response, response, err)
	return
}
