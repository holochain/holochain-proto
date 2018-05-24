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
