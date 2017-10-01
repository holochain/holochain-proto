// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Ribosome provides an interface for an execution environment interface for chains and their entries
// and factory code for creating ribosome instances

package holochain

import (
	"errors"
	"fmt"
	. "github.com/metacurrency/holochain/hash"
	"sort"
	"strings"
)

type RibosomeFactory func(h *Holochain, zome *Zome) (Ribosome, error)

const (

	// calling types

	STRING_CALLING = "string"
	JSON_CALLING   = "json"

	// exposure types for functions

	// ZOME_EXPOSURE is the default and means the function is only exposed for use by other zomes in the app
	ZOME_EXPOSURE = ""
	// AUTHENTICATED_EXPOSURE means that the function is only available after authentication (TODO)
	AUTHENTICATED_EXPOSURE = "auth"
	// PUBLIC_EXPOSURE means that the function is callable by anyone
	PUBLIC_EXPOSURE = "public"

	// these constants are for a removed feature, see ChangeAppProperty
	// @TODO figure out how to remove code over time that becomes obsolete, i.e. for long-dead changes

	ID_PROPERTY         = "_id"
	AGENT_ID_PROPERTY   = "_agent_id"
	AGENT_NAME_PROPERTY = "_agent_name"

	BridgeFrom    = 0
	BridgeTo      = 1
	BridgeFromStr = "0"
	BridgeToStr   = "1"
)

var ValidationFailedErr = errors.New("Validation Failed")

// FunctionDef holds the name and calling type of an DNA exposed function
type FunctionDef struct {
	Name        string
	CallingType string
	Exposure    string
}

// ValidExposure verifies that the function can be called in the given context
func (f *FunctionDef) ValidExposure(context string) bool {
	if f.Exposure == PUBLIC_EXPOSURE {
		return true
	}
	return f.Exposure == context
}

// Ribosome type abstracts the functions of code execution environments
type Ribosome interface {
	Type() string
	ValidateAction(action Action, def *EntryDef, pkg *ValidationPackage, sources []string) (err error)
	ValidatePackagingRequest(action ValidatingAction, def *EntryDef) (req PackagingReq, err error)
	ChainGenesis() error
	BridgeGenesis(side int, dnaHash Hash, data string) error
	Receive(from string, msg string) (response string, err error)
	Call(fn *FunctionDef, params interface{}) (interface{}, error)
	Run(code string) (result interface{}, err error)
	RunAsyncSendResponse(response AppMsg, callback string, callbackID string) (result interface{}, err error)
}

var ribosomeFactories = make(map[string]RibosomeFactory)

// RegisterRibosome sets up a Ribosome to be used by the CreateRibosome function
func RegisterRibosome(name string, factory RibosomeFactory) {
	if factory == nil {
		panic(fmt.Sprintf("Ribosome factory for type %s does not exist.", name))
	}
	_, registered := ribosomeFactories[name]
	if registered {
		panic(fmt.Sprintf("Ribosome factory for type %s already registered. ", name))
	}
	ribosomeFactories[name] = factory
}

// RegisterBultinRibosomes adds the built in ribosome types to the factory hash
func RegisterBultinRibosomes() {
	RegisterRibosome(ZygoRibosomeType, NewZygoRibosome)
	RegisterRibosome(JSRibosomeType, NewJSRibosome)
}

// CreateRibosome returns a new Ribosome of the given type
func CreateRibosome(h *Holochain, zome *Zome) (Ribosome, error) {

	factory, ok := ribosomeFactories[zome.RibosomeType]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available ribosome factories for error.
		var available []string
		for k := range ribosomeFactories {
			available = append(available, k)
		}
		sort.Strings(available)
		return nil, fmt.Errorf("Invalid ribosome name. Must be one of: %s", strings.Join(available, ", "))
	}

	return factory(h, zome)
}
