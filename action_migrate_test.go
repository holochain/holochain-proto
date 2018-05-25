package holochain

import (
	. "github.com/holochain/holochain-proto/hash"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestMigrateName(t *testing.T) {
	var chain Hash
	var user Hash
	Convey("migrate action should have the right name", t, func() {
		a := ActionMigrate{entry: MigrateEntry{Chain: chain, User: user}}
		So(a.Name(), ShouldEqual, "migrate")
	})
}

func TestAPIFnMigrateName(t *testing.T) {
	var chain Hash
	var user Hash
	Convey("migrate action function should have the right name", t, func() {
		a := ActionMigrate{entry: MigrateEntry{Chain: chain, User: user}}
		fn := &APIFnMigrate{action: a}
		So(fn.Name(), ShouldEqual, "migrate")
	})
}

func TestMigrateEntry(t *testing.T) {
	Convey("empty migrate action Entry() should be retreive a serialized JSON of an empty entry in a GobEntry", t, func() {
		action := ActionMigrate{}
		So(action.Entry(), ShouldResemble, &GobEntry{C: "{\"Chain\":\"\",\"User\":\"\",\"Data\":\"\"}"})
	})

	Convey("entries with vals work with Entry()", t, func() {
		chain := genTestHashString()
		user := genTestHashString()
		entry := MigrateEntry{Chain: chain, User: user}
		action := ActionMigrate{entry: entry}

		So(action.Entry(), ShouldResemble, &GobEntry{C: "{\"Chain\":\"" + string(chain) + "\",\"User\":\"" + string(user) + "\",\"Data\":\"\"}"})
	})
}

func TestMigrateEntryType(t *testing.T) {
	action := ActionMigrate{}
	Convey("migrate action EntryType() should return the correct type", t, func() {
		So(action.EntryType(), ShouldEqual, MigrateEntryType)
	})
}

func TestMigrateHeaderSetGet(t *testing.T) {
	Convey("empty migrate action should have empty header", t, func() {
		action := ActionMigrate{}
		So(action.GetHeader(), ShouldEqual, nil)
	})
}
