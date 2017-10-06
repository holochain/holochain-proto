// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

package holochain

import (
	"errors"
	. "github.com/metacurrency/holochain/hash"
)

// Zome struct encapsulates logically related code, from a "chromosome"
type Zome struct {
	Name         string
	Description  string
	Code         string
	Entries      []EntryDef
	RibosomeType string
	Functions    []FunctionDef
	BridgeFuncs  []string // functions in zome that can be bridged to by fromApp
	BridgeTo     Hash     // dna Hash of toApp that this zome is a client of
}

// GetEntryDef returns the entry def structure
func (z *Zome) GetEntryDef(entryName string) (e *EntryDef, err error) {
	for _, def := range z.Entries {
		if def.Name == entryName {
			e = &def
			break
		}
	}
	if e == nil {
		err = errors.New("no definition for entry type: " + entryName)
	}
	return
}

func (z *Zome) GetPrivateEntryDefs() (privateDefs []EntryDef) {
	privateDefs = make([]EntryDef, 0)
	for _, def := range z.Entries {
		if def.Sharing == "private" {
			privateDefs = append(privateDefs, def)
		}
	}
	return
}

// GetFunctionDef returns the exposed function spec for the given zome and function
func (zome *Zome) GetFunctionDef(fnName string) (fn *FunctionDef, err error) {
	for _, f := range zome.Functions {
		if f.Name == fnName {
			fn = &f
			break
		}
	}
	if fn == nil {
		err = errors.New("unknown exposed function: " + fnName)
	}
	return
}

func (zome *Zome) MakeRibosome(h *Holochain) (r Ribosome, err error) {
	r, err = CreateRibosome(h, zome)
	return
}

func (zome *Zome) CodeFileName() string {
	if zome.RibosomeType == ZygoRibosomeType {
		return zome.Name + ".zy"
	} else if zome.RibosomeType == JSRibosomeType {
		return zome.Name + ".js"
	}
	panic("unknown ribosome type:" + zome.RibosomeType)
}
