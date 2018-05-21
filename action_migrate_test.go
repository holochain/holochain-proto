package holochain

import (
  . "github.com/smartystreets/goconvey/convey"
  "testing"
)

func TestMigrateName(t *testing.T) {
  Convey("migrate action should have the right name", t, func() {
    a := ActionMigrate{entry: MigrateEntry{Hash: ""}}
    So(a.Name(), ShouldEqual, "migrate")
  })
}

func TestAPIFnMigrateName(t *testing.T) {
  Convey("migrate action function should have the right name", t, func() {
    a := ActionMigrate{entry: MigrateEntry{Hash: ""}}
    fn := &APIFnMigrate{action: a}
    So(fn.Name(), ShouldEqual, "migrate")
  })
}
