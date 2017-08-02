package holochain

import (
	. "github.com/smartystreets/goconvey/convey"
	"github.com/tidwall/buntdb"
	"path/filepath"
	"testing"
)

func TestCapabilitiesGeneral(t *testing.T) {
	d := SetupTestDir()
	defer CleanupTestDir(d)

	db, err := buntdb.Open(filepath.Join(d, "test_cap_db"))
	if err != nil {
		panic(err)
	}

	var c *Capability
	capabilityType := "capability identifier"
	c, err = NewCapability(db, capabilityType, nil)
	Convey("it should create a general capability", t, func() {
		So(err, ShouldBeNil)
		So(c.db, ShouldEqual, db)
		So(c.Token, ShouldNotEqual, "")
	})

	Convey("it should validate a general capability", t, func() {
		capType, err := c.Validate(nil)
		So(err, ShouldBeNil)
		So(capType, ShouldEqual, capabilityType)
	})

	Convey("it should not validate a manufactured bogus token", t, func() {
		badC := Capability{Token: "bogus", db: db}
		_, err = badC.Validate(nil)
		So(err, ShouldEqual, CapabilityInvalidErr)
	})

	Convey("it should produce multiple tokens of the same type", t, func() {
		c2, err := NewCapability(db, capabilityType, nil)
		So(err, ShouldBeNil)
		So(c2.Token, ShouldNotEqual, c.Token)
	})

	Convey("it should not validate a revoked capability", t, func() {
		err = c.Revoke(nil)
		So(err, ShouldBeNil)
		_, err = c.Validate(nil)
		So(err, ShouldEqual, CapabilityInvalidErr)
	})

}
