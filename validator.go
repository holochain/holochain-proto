// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Validator implements a validation engine interface for chains and their entries
// additionally it implements a zygomys use of that interface

package holochain

import (
	"errors"
	"fmt"
	zygo "github.com/glycerine/zygomys/repl"
)

const (
	ZygoSchemaType = "zygo"
)

type Validator interface {
	Name() string
	ValidateEntry(entry interface{}) error
}

type ZygoValidator struct {
	env *zygo.Glisp
}

func (z *ZygoValidator) Name() string { return ZygoSchemaType }

func (z *ZygoValidator) ValidateEntry(entry interface{}) (err error) {
	e := entry.(string)
	err = z.env.LoadString("(validateEntry " + e + ")")
	if err != nil {
		return
	}
	result, err := z.env.Run()
	switch result.(type) {
	case *zygo.SexpBool:
		r := result.(*zygo.SexpBool).Val
		if !r {
			err = errors.New("Invalid entry:" + e)
		}
	default:
		err = errors.New("Unexpected result: " + fmt.Sprintf("%v", result))
	}
	return
}

func NewZygoValidator(code string) (v Validator, err error) {
	var z ZygoValidator
	z.env = zygo.NewGlisp()
	err = z.env.LoadString(code)
	if err != nil {
		err = errors.New("Zygomys error: " + err.Error())
		return
	}
	v = &z
	return
}
