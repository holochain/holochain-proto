// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// change implements adding of features to holochain such that deprecation and version dependency
// is knowable by app developers

package holochain

import ()

type ChangeType int8

const (
	Deprecation ChangeType = iota
	Warning
)

// Change represents a semantic change that needs to be reported
type Change struct {
	Type    ChangeType
	Message string
	AsOf    int
}

func (c *Change) Log() {
	var h string
	switch c.Type {
	case Deprecation:
		h = "Deprecation warning: "
	case Warning:
		h = "Warning: "
	}
	log.Debugf(h+c.Message, c.AsOf)
}

var ChangeAppProperty = Change{
	Type:    Deprecation,
	Message: "Getting special properties via property() is deprecated as of %d",
	AsOf:    2,
}

var ChangeRequires = Change{
	Type:    Warning,
	Message: "Zomes must define 'requires' function as of version %d, assuming no requirements.",
	AsOf:    2,
}
