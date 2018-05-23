// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
//
package holochain

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
