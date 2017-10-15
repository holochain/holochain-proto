// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// implements the abstractions and functions for application bridging
//

package holochain

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/metacurrency/holochain/hash"
	"github.com/tidwall/buntdb"
	"path/filepath"
	"strings"
)

// BridgeApp describes an app for bridging, used
type BridgeApp struct {
	H                     *Holochain
	Side                  int
	BridgeGenesisDataFrom string
	BridgeGenesisDataTo   string
	Port                  string // only used if side == BridgeTo
}

// Bridge holds data returned by GetBridges
type Bridge struct {
	ToApp Hash
	Token string
	Side  int
}

type BridgeSpec map[string]map[string]bool

var BridgeAppNotFoundErr = errors.New("bridge app not found")

// AddBridgeAsCallee registers a token for allowing bridged calls from some other app
// and calls bridgeGenesis in any zomes with bridge functions
func (h *Holochain) AddBridgeAsCallee(fromDNA Hash, appData string) (token string, err error) {
	h.Debugf("Adding bridge to %s from %v with appData: %s", h.Name(), fromDNA, appData)
	err = h.initBridgeDB()
	if err != nil {
		return
	}
	var capability *Capability

	bridgeSpec := h.makeBridgeSpec()
	var bridgeSpecB []byte

	if bridgeSpec != nil {
		bridgeSpecB, err = json.Marshal(bridgeSpec)
		if err != nil {
			return
		}
	}

	capability, err = NewCapability(h.bridgeDB, string(bridgeSpecB), nil)
	if err != nil {
		return
	}

	for zomeName, _ := range bridgeSpec {
		var r Ribosome
		r, _, err = h.MakeRibosome(zomeName)
		if err != nil {
			return
		}
		h.Debugf("Running BridgeTo Genesis for %s", zomeName)
		err = r.BridgeGenesis(BridgeTo, fromDNA, appData)
		if err != nil {
			return
		}
	}

	token = capability.Token

	return
}

func (h *Holochain) initBridgeDB() (err error) {
	if h.bridgeDB == nil {
		h.bridgeDB, err = buntdb.Open(filepath.Join(h.DBPath(), BridgeDBFileName))
	}
	return
}

func checkBridgeSpec(spec BridgeSpec, zomeType string, function string) bool {
	f, ok := spec[zomeType]
	if ok {
		_, ok = f[function]
	}
	return ok
}

func (h *Holochain) makeBridgeSpec() (spec BridgeSpec) {
	var funcs map[string]bool
	for _, z := range h.nucleus.dna.Zomes {
		for _, f := range z.BridgeFuncs {
			if spec == nil {
				spec = make(BridgeSpec)
			}
			_, ok := spec[z.Name]
			if !ok {
				funcs = make(map[string]bool)
				spec[z.Name] = funcs

			}
			funcs[f] = true
		}
	}
	return
}

// BridgeCall executes a function exposed through a bridge
func (h *Holochain) BridgeCall(zomeType string, function string, arguments interface{}, token string) (result interface{}, err error) {
	if h.bridgeDB == nil {
		err = errors.New("no active bridge")
		return
	}
	c := Capability{Token: token, db: h.bridgeDB}

	var bridgeSpecStr string
	bridgeSpecStr, err = c.Validate(nil)
	if err == nil {
		if bridgeSpecStr != "*" {
			bridgeSpec := make(BridgeSpec)
			err = json.Unmarshal([]byte(bridgeSpecStr), &bridgeSpec)
			if err == nil {
				if !checkBridgeSpec(bridgeSpec, zomeType, function) {
					err = errors.New("function not bridged")
					return
				}
			}
		}
		if err == nil {
			result, err = h.Call(zomeType, function, arguments, ZOME_EXPOSURE)
		}
	}

	if err != nil {
		err = errors.New("bridging error: " + err.Error())

	}

	return
}

// AddBridgeAsCaller associates a token with an application DNA hash and url for accessing it
// it also runs BridgeGenesis for the From side
func (h *Holochain) AddBridgeAsCaller(toDNA Hash, token string, url string, appData string) (err error) {
	h.Debugf("Adding bridge from %s to %v with appData: %s", h.Name(), toDNA, appData)
	err = h.initBridgeDB()
	if err != nil {
		return
	}
	toDNAStr := toDNA.String()
	err = h.bridgeDB.Update(func(tx *buntdb.Tx) error {
		_, _, err = tx.Set("app:"+toDNAStr, token, nil)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("url:"+toDNAStr, url, nil)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	var bridged bool
	// TODO  possible that we shouldn't add the bridge unless the there is some Zome with BridgeTo?
	// the way this is is just that the only way to get the from genesis to run is if it's set
	for _, z := range h.nucleus.dna.Zomes {
		if z.BridgeTo.String() == toDNAStr {
			var r Ribosome
			r, _, err = h.MakeRibosome(z.Name)
			if err != nil {
				return
			}
			h.Debugf("Running BridgeFrom Genesis for %s", z.Name)
			err = r.BridgeGenesis(BridgeFrom, toDNA, appData)
			if err != nil {
				return
			}
			bridged = true
		}
	}
	if !bridged {
		Infof("Warning: no zome called for bridging to: %v", toDNA)
	}
	return
}

// GetBridgeToken returns a token given the a hash
func (h *Holochain) GetBridgeToken(hash Hash) (token string, url string, err error) {
	if h.bridgeDB == nil {
		err = errors.New("no active bridge")
		return
	}
	err = h.bridgeDB.View(func(tx *buntdb.Tx) (e error) {
		token, e = tx.Get("app:" + hash.String())
		if e == buntdb.ErrNotFound {
			e = BridgeAppNotFoundErr
		}
		url, e = tx.Get("url:" + hash.String())
		if e == buntdb.ErrNotFound {
			e = BridgeAppNotFoundErr
		}
		return
	})
	h.Debugf("found bridge token %s with url %s for %s", token, url, hash.String())
	return
}

// BuildBridge creates the bridge structures on both sides
// assumes that GenChain has been called for both sides already
func (h *Holochain) BuildBridge(app *BridgeApp, port string) (err error) {
	var hFrom, hTo *Holochain
	var toPort string
	if app.Side == BridgeFrom {
		hFrom = app.H
		hTo = h
		toPort = port
	} else {
		hTo = app.H
		hFrom = h
		toPort = app.Port
	}

	var token string
	token, err = hTo.AddBridgeAsCallee(hFrom.DNAHash(), app.BridgeGenesisDataTo)
	if err != nil {
		h.Debugf("adding bridge to %s from %s failed with %v\n", hTo.Name(), hFrom.Name(), err)
		return
	}
	h.Debugf("%s received token %s from %s\n", hFrom.Name(), token, hTo.Name())

	// the url is currently through the webserver
	err = hFrom.AddBridgeAsCaller(hTo.DNAHash(), token, fmt.Sprintf("http://localhost:%s", toPort), app.BridgeGenesisDataFrom)
	if err != nil {
		h.Debugf("adding bridge from %s to %s failed with %s\n", hFrom.Name(), hTo.Name(), err)
		return
	}

	return
}

// GetBridges returns a list of the active bridges on the holochain
func (h *Holochain) GetBridges() (bridges []Bridge, err error) {
	if h.bridgeDB == nil {
		bridgeDBFile := filepath.Join(h.DBPath(), BridgeDBFileName)
		if FileExists(bridgeDBFile) {
			h.bridgeDB, err = buntdb.Open(bridgeDBFile)
			if err != nil {
				return
			}
		}
	}
	if h.bridgeDB != nil {
		err = h.bridgeDB.View(func(tx *buntdb.Tx) error {
			err = tx.Ascend("", func(key, value string) bool {
				x := strings.Split(key, ":")
				var hash Hash
				switch x[0] {
				case "app":
					hash, err = NewHash(x[1])
					if err != nil {
						return false
					}
					bridges = append(bridges, Bridge{ToApp: hash, Side: BridgeFrom})
				case "tok":
					bridges = append(bridges, Bridge{Token: x[1], Side: BridgeTo})
				}
				return true
			})
			return err
		})
	}
	return
}
