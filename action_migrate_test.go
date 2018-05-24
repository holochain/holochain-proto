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
  var chain Hash
  var user Hash
  entry := MigrateEntry{Chain: chain, User: user}
  action := ActionMigrate{entry: entry}
  Convey("migrate action Entry() should be retreive a serialized JSON of the entry in a GobEntry", t, func() {
    var jsonEntry, _ = entry.ToJSON()
    So(action.Entry(), ShouldResemble, &GobEntry{C: jsonEntry})
  })
}
