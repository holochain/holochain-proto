package holochain

import (
  "testing"
  . "github.com/smartystreets/goconvey/convey"
  "fmt"
  // "log"
)

// func TestMigrateEntryFromJSON(t *testing.T) {
//   jsonString := "{\"Type\":\"close\",\"Chain\":\"6NzcqKNXBw8UCNaKeMvoQd3xyCwkuwCvw4B6pgZ334nhVyyjDE3tH3BTLsrLnvA\",\"User\":\"6Nzcp9ELoTyHB2auSE1Pj3uNepPnqWbf2bU4RBsXZi35XZB4uGkdweeNPeCRwnA\",\"Data\":\"6NzYZjEtV8c9Axz5k8XrWfMqGzDL5Wi8nV7qMczo4aU3FhZTfKhDxGgCmDs7hfd\"}"
//   entry, err := MigrateEntryFromJSON(jsonString)
//   if err != nil {
//     panic(err)
//   }
//   j, _ := entry.ToJSON()
//   log.Print(j)
// }

func TestMigrateEntryToJSON(t *testing.T) {
  chain, err := genTestStringHash()
  if err != nil {
    panic(err)
  }
  user, err := genTestStringHash()
  if err != nil {
    panic(err)
  }
  data, err := genTestString()
  if err != nil {
    panic(err)
  }
  // Note that the k/v order here is different from the resulting JSON.
  // This is deliberate to test that k/v order in code does not influence data
  // output at runtime.
  entry := MigrateEntry{
    User: user,
    Chain: chain,
    Type: "open",
    Data: data,
  }

  Convey("MigrateEntry should convert to JSON and roundtrip safely", t, func() {
    var j string
    var err error
		j, err = entry.ToJSON()

    if err != nil {
      panic(err)
    }

    So(user, ShouldNotEqual, chain)
    So(user, ShouldNotEqual, data)
    So(chain, ShouldNotEqual, data)
		So(err, ShouldBeNil)
		So(j, ShouldEqual, fmt.Sprintf(`{"Type":"open","Chain":"%s","User":"%s","Data":"%s"}`, chain, user, data))

    roundtrip, err := MigrateEntryFromJSON(j)
    if err != nil {
      panic(err)
    }
    So(roundtrip.User, ShouldEqual, user)
    So(roundtrip, ShouldResemble, entry)
	})
}
