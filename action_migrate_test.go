package holochain

import (
  . "github.com/smartystreets/goconvey/convey"
  "testing"
  . "github.com/holochain/holochain-proto/hash"
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
  entry := MigrateEntry{}
  var emptyJSONEntry, _ = entry.ToJSON()
  Convey("migrate action Entry() should be retreive a serialized JSON of the entry in a GobEntry", t, func() {
    action := ActionMigrate{}
    So(action.Entry(), ShouldResemble, &GobEntry{C: emptyJSONEntry})
  })
}

func TestEntryType(t *testing.T) {
  action := ActionMigrate{}
  Convey("migrate action EntryType() should return the correct type", t, func() {
    So(action.EntryType(), ShouldEqual, MigrateEntryType)
  })
}
