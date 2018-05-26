package holochain

import (
  "testing"
  . "github.com/holochain/holochain-proto/hash"
  . "github.com/smartystreets/goconvey/convey"
  "fmt"
)

func TestMigrateEntrySysValidation(t *testing.T) {
  d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

  Convey("it should validate against JSON", t, func() {
		entry := MigrateEntry{}
    action := ActionMigrate{entry: entry}
		err := sysValidateEntry(h, entry.Def(), action.Entry(), nil)
		So(err, ShouldBeNil)
	})
}

func TestMigrateEntryFromJSON(t *testing.T) {
  jsonString := "{\"Type\":\"open\",\"Chain\":\"1AarHJii5CkF6waPp4e3VgniqYB5byyyb5sWzewxvBUsPN\",\"User\":\"1AeVYmanHKEJP36WjvV7ZBzhBR9F8euDd2ejJLTdxbAtD2\",\"Data\":\"1AiydpQZ57G8LAamezKFySyy2DKghX3q83ZDMnqnSp5Vyi\"}"
  entry, err := MigrateEntryFromJSON(jsonString)
  if err != nil {
    panic(err)
  }

  Convey("MigrateEntry should be unserializable from JSON", t, func() {
    unserializedChain, err := NewHash("1AarHJii5CkF6waPp4e3VgniqYB5byyyb5sWzewxvBUsPN")
    unserializedUser, err := NewHash("1AeVYmanHKEJP36WjvV7ZBzhBR9F8euDd2ejJLTdxbAtD2")
    if err != nil {
      panic(err)
    }

    So(entry.Chain, ShouldEqual, unserializedChain)
    So(entry.User, ShouldEqual, unserializedUser)
    So(entry.Data, ShouldEqual, "1AiydpQZ57G8LAamezKFySyy2DKghX3q83ZDMnqnSp5Vyi")
    So(entry.Type, ShouldEqual, "open")
  })
}

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
