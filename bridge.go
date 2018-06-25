// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// implements the abstractions and functions for application bridging
//

package holochain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/HC-Interns/holochain-proto/hash"
	"github.com/tidwall/buntdb"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
)

// BridgeApp describes a data necessary for bridging
type BridgeApp struct {
	Name                    string //Name of other side
	DNA                     Hash   // DNA of other side
	Side                    int
	BridgeGenesisCallerData string
	BridgeGenesisCalleeData string
	Port                    string // only used if side == BridgeCallee
	BridgeZome              string // only used if side == BridgeCaller
}

// Bridge holds data returned by GetBridges
type Bridge struct {
	CalleeApp  Hash
	CalleeName string
	Token      string
	Side       int
}

type BridgeSpec map[string]map[string]bool

var BridgeAppNotFoundErr = errors.New("bridge app not found")

// AddBridgeAsCallee registers a token for allowing bridged calls from some other app
// and calls bridgeGenesis in any zomes with bridge functions
func (h *Holochain) AddBridgeAsCallee(fromDNA Hash, appData string) (token string, err error) {
	h.Debugf("Adding bridge to callee %s from caller %v with appData: %s", h.Name(), fromDNA, appData)
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
		h.Debugf("Running BridgeCallee Genesis for %s", zomeName)
		err = r.BridgeGenesis(BridgeCallee, fromDNA, appData)
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
// it also runs BridgeGenesis in the bridgeZome
func (h *Holochain) AddBridgeAsCaller(bridgeZome string, calleeDNA Hash, calleeName string, token string, url string, appData string) (err error) {
	h.Debugf("Adding bridge to caller %s for callee %s (%v) with appData: %s", h.Name(), calleeName, calleeDNA, appData)
	err = h.initBridgeDB()
	if err != nil {
		return
	}
	toDNAStr := calleeDNA.String()
	err = h.bridgeDB.Update(func(tx *buntdb.Tx) error {
		_, _, err = tx.Set("app:"+toDNAStr, token+"%%"+url+"%%"+calleeName, nil)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	var zome *Zome

	// get the zome that does the bridging, as we need to run the bridgeGenesis function in it
	zome, err = h.GetZome(bridgeZome)
	if err != nil {
		err = fmt.Errorf("error getting bridging zome: %v", err)
		return
	}
	var r Ribosome
	r, _, err = h.MakeRibosome(zome.Name)
	if err != nil {
		return
	}

	h.Debugf("Running BridgeCaller Genesis for %s", zome.Name)
	err = r.BridgeGenesis(BridgeCaller, calleeDNA, appData)
	if err != nil {
		return
	}
	return
}

func getBridgeAppVals(value string) (token string, url string, name string) {
	x := strings.Split(value, "%%")
	token = x[0]
	url = x[1]
	name = x[2]
	return
}

// GetBridgeToken returns a token given the a hash
func (h *Holochain) GetBridgeToken(hash Hash) (token string, url string, err error) {
	if h.bridgeDB == nil {
		err = errors.New("no active bridge")
		return
	}
	err = h.bridgeDB.View(func(tx *buntdb.Tx) (e error) {
		var value string
		value, e = tx.Get("app:" + hash.String())
		if e == buntdb.ErrNotFound {
			e = BridgeAppNotFoundErr
		}
		if e == nil {
			token, url, _ = getBridgeAppVals(value)
		}
		return
	})
	h.Debugf("found bridge token %s with url %s for %s", token, url, hash.String())
	return
}

// BuildBridgeToCaller connects h to a running app specified by BridgeApp that will be the Caller, i.e. the the BridgeCaller
func (h *Holochain) BuildBridgeToCaller(app *BridgeApp, port string) (err error) {
	var token string
	token, err = h.AddBridgeAsCallee(app.DNA, app.BridgeGenesisCalleeData)
	if err != nil {
		h.Debugf("adding bridge to caller %s from %s failed with %v\n", app.Name, h.Name(), err)
		return
	}

	h.Debugf("%s generated token %s for %s\n", h.Name(), token, app.Name)

	data := map[string]string{"Type": "ToCaller", "Zome": app.BridgeZome, "DNA": h.DNAHash().String(), "Token": token, "Port": port, "Data": app.BridgeGenesisCallerData}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return
	}

	body := bytes.NewBuffer(dataJSON)
	var resp *http.Response

	resp, err = http.Post(fmt.Sprintf("http://0.0.0.0:%s/setup-bridge/", app.Port), "application/json", body)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			err = errors.New(resp.Status)
		}
	}
	if err != nil {
		h.Debugf("adding bridge to caller %s from %s failed with %s\n", app.Name, h.Name(), err)
	}
	return
}

// BuildBridgeToCallee connects h to a running app specified by BridgeApp that will be the Callee, i.e. the the BridgeCallee
func (h *Holochain) BuildBridgeToCallee(app *BridgeApp) (err error) {

	data := map[string]string{"Type": "ToCallee", "DNA": h.DNAHash().String(), "Data": app.BridgeGenesisCalleeData}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return
	}
	body := bytes.NewBuffer(dataJSON)
	var resp *http.Response
	resp, err = http.Post(fmt.Sprintf("http://0.0.0.0:%s/setup-bridge/", app.Port), "application/json", body)

	if err == nil {
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			err = errors.New(resp.Status)
		}
	}
	if err != nil {
		return
	}

	var b []byte
	b, err = ioutil.ReadAll(resp.Body)

	if err != nil {
		h.Debugf("adding bridge to callee %s from %s failed with %v\n", app.Name, h.Name(), err)
		return
	}

	token := string(b)
	h.Debugf("%s received token %s from %s\n", h.Name(), token, app.Name)

	// the url is currently through the webserver
	err = h.AddBridgeAsCaller(app.BridgeZome, app.DNA, app.Name, token, fmt.Sprintf("http://localhost:%s", app.Port), app.BridgeGenesisCallerData)
	if err != nil {
		h.Debugf("adding bridge to callee %s from %s failed with %s\n", app.Name, h.Name(), err)
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
					_, _, name := getBridgeAppVals(value)
					bridges = append(bridges, Bridge{CalleeApp: hash, CalleeName: name, Side: BridgeCaller})
				case "tok":
					bridges = append(bridges, Bridge{Token: x[1], Side: BridgeCallee})
				}
				return true
			})
			return err
		})
	}
	return
}
