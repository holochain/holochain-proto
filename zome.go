// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

package holochain

import (
	"errors"
)

// Zome struct encapsulates logically related code, from a "chromosome"
type Zome struct {
	Name         string
	Description  string
	Code         string // file name of DNA code
	CodeHash     Hash
	Entries      []EntryDef
	RibosomeType string
	Functions    []FunctionDef

	// cache for code
	code string
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
