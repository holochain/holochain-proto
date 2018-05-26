package holochain

import (
  "testing"
  . "github.com/smartystreets/goconvey/convey"
  "fmt"
  "log"
)

func TestMigrateEntryToJSON(t *testing.T) {
  chain := genTestStringHash()
  user := genTestStringHash()
  data := genTestString()
  entry := MigrateEntry{
    Type: "open",
    Chain: chain,
    User: user,
    Data: data,
  }

  log.Print(user.String())
  log.Print(chain.String())

  Convey("MigrateEntry should convert to JSON", t, func() {
    var j string
    var err error
		j, err = entry.ToJSON()
		So(err, ShouldBeNil)
		So(j, ShouldEqual, fmt.Sprintf(`{"Type":"open","Chain":"%s","User":"%s","Data":"%s"}`, chain, user, data))
	})
}

func TestMigrateEntryFromJSON(t *testing.T) {
  // @TODO
}
