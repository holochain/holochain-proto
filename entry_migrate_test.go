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

  Convey("validate MigrateEntry against JSON", t, func() {
    toEntry := func (entry MigrateEntry) (e Entry) {
      action := ActionMigrate{entry: entry}
      return action.Entry()
    }
    var err error
		entry := MigrateEntry{}

    err = sysValidateEntry(h, entry.Def(), toEntry(entry), nil)
		So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding Chain value ''")

    chain, err := genTestStringHash()
    user, err := genTestStringHash()
    migrateType := randomSliceItem([]string{MigrateEntryTypeOpen, MigrateEntryTypeClose})
    data, err := genTestString()

    entry.Chain = chain
    err = sysValidateEntry(h, entry.Def(), toEntry(entry), nil)
    So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding User value ''")

    entry.User = user
    err = sysValidateEntry(h, entry.Def(), toEntry(entry), nil)
    So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: Type value '' must be either 'open' or 'close'")

    entry.Type = migrateType
    So(sysValidateEntry(h, entry.Def(), toEntry(entry), nil), ShouldBeNil)

    entry.Data = data
    So(sysValidateEntry(h, entry.Def(), toEntry(entry), nil), ShouldBeNil)

    emptyString := &GobEntry{C: ""}
    err = sysValidateEntry(h, MigrateEntryDef, emptyString, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "unexpected end of JSON input")

    missingType := &GobEntry{C: "{\"Chain\":\"1AaJq9cCYEBEZEbfmwupdb51gG8yZr9LTBxhBeXSZJtJbA\",\"User\":\"1AnJDazAvUmNH6rzxQxGho1fBhd1kxfWjJJ8rkrbbDarb1\",\"Data\":\"1Akcx6p98n5FaSgxF8h7s8mdiua6JkctjLtLsEsSaSHVZn\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, missingType, nil)
    So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: validator %migrate failed: object property 'Type' is required")

    missingChain := &GobEntry{C: "{\"Type\":\"1AZJizDv7dKiSm5umS2muVoK4GCVm9jPCGidSndczyE64b\",\"User\":\"1AncHr4PvHbkYNW4jdgmqJWfMArcAndLRrVGwVW18dtUN1\",\"Data\":\"1AjaEHtBfb9vLEsivCsPHH5NyBuEwrbkzzK8w54ufCFXw5\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, missingChain, nil)
    So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: validator %migrate failed: object property 'Chain' is required")

    missingUser := &GobEntry{C: "{\"Type\":\"1AoLAq5VA5rT5tPBKsmSksGc7b4avtF2gjhGxCwrJV4hpi\",\"Chain\":\"1AWteDFmYMHHyZ3BRM4ndTLXaeoKiuwTyNkCVHVv4KTZ3p\",\"Data\":\"1AmDjukyX7B5Kh57DDKE8MzNfLUUF5n7nBpuGBi61njRPr\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, missingUser, nil)
    So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: validator %migrate failed: object property 'User' is required")

    brokenChain := &GobEntry{C: "{\"Type\":\"1AgHrybioSgRuMGVvkD6NjqBiCmpap3gAKgGcgzaBodXE9\",\"Chain\":\"not-a-hash\",\"User\":\"1AYFPBzgLWVGEy2MFSY9ZyLw7c224fWykZKy3HWx32SJrC\",\"Data\":\"1AmXqXdBCVcraVaWB3sk7HemHWq5wkCZX1GW3fPDgj3Htz\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, brokenChain, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding Chain value 'not-a-hash'")

    brokenUser := &GobEntry{C: "{\"Type\":\"1AgHrybioSgRuMGVvkD6NjqBiCmpap3gAKgGcgzaBodXE9\",\"User\":\"not-a-hash\",\"Chain\":\"1AYFPBzgLWVGEy2MFSY9ZyLw7c224fWykZKy3HWx32SJrC\",\"Data\":\"1AmXqXdBCVcraVaWB3sk7HemHWq5wkCZX1GW3fPDgj3Htz\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, brokenUser, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding User value 'not-a-hash'")
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
