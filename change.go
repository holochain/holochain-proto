// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// change implements adding of features to holochain such that deprecation and version dependency
// is knowable by app developers

package holochain

import ()

var ChangeAppProperty = Change{
	DeprecationMessage: "Getting special properties via property() is deprecated as of %s",
	AsOf:               "0.0.2",
}

// Change represents a semantic change that needs to be reported
type Change struct {
	DeprecationMessage string
	AsOf               string
}

func (c *Change) Deprecated() {
	log.Debugf("Deprecation warning: "+c.DeprecationMessage, c.AsOf)
}
