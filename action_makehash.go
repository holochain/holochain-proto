// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

import (
	. "github.com/HC-Interns/holochain-proto/hash"
)

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
