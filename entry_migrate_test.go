package holochain

import (
  "testing"
  . "github.com/smartystreets/goconvey/convey"
  "fmt"
)

func TestMigrateConstants(t *testing.T) {
  Convey("migrate constants should have the right values", t, func() {
		So(MigrateEntryType, ShouldEqual, "%migrate")
		So(MigrateEntryTypeClose, ShouldEqual, "close")
		So(MigrateEntryTypeOpen, ShouldEqual, "open")
	})
}

func TestMigrateEntryDef(t *testing.T) {
  entry := MigrateEntry{}
  Convey("validate MigrateEntryDef properties", t, func() {
    So(entry.Def().Name, ShouldEqual, MigrateEntryType)
    So(entry.Def().DataFormat, ShouldEqual, DataFormatJSON)
    So(entry.Def().Sharing, ShouldEqual, Public)
  })
}

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
    So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding DNAHash value ''")

    dnaHash, err := genTestStringHash()
    key, err := genTestStringHash()
    So(err, ShouldBeNil)

    migrateType := randomSliceItem([]string{MigrateEntryTypeOpen, MigrateEntryTypeClose})
    data, err := genTestString()
    So(err, ShouldBeNil)

    entry.DNAHash = dnaHash

    err = sysValidateEntry(h, entry.Def(), toEntry(entry), nil)
    So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding Key value ''")

    entry.Key = key
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

    missingType := &GobEntry{C: "{\"DNAHash\":\"" + dnaHash.String() + "\",\"Key\":\"" + key.String() + "\",\"Data\":\"" + data + "\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, missingType, nil)
    So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: validator %migrate failed: object property 'Type' is required")

    missingDNAHash := &GobEntry{C: "{\"Type\":\"" + migrateType + "\",\"Key\":\"" + key.String() + "\",\"Data\":\"" + data + "\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, missingDNAHash, nil)
    So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: validator %migrate failed: object property 'DNAHash' is required")

    missingKey := &GobEntry{C: "{\"Type\":\"" + migrateType + "\",\"DNAHash\":\"" + dnaHash.String() + "\",\"Data\":\"" + data + "\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, missingKey, nil)
    So(err, ShouldNotBeNil)
    So(err.Error(), ShouldEqual, "Validation Failed: validator %migrate failed: object property 'Key' is required")

    brokenDNAHash := &GobEntry{C: "{\"Type\":\"" + migrateType + "\",\"DNAHash\":\"not-a-hash\",\"Key\":\"" + key.String() + "\",\"Data\":\"" + data + "\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, brokenDNAHash, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding DNAHash value 'not-a-hash'")

    brokenKey := &GobEntry{C: "{\"Type\":\"" + migrateType + "\",\"Key\":\"not-a-hash\",\"DNAHash\":\"" + dnaHash.String() + "\",\"Data\":\"" + data + "\"}"}
    err = sysValidateEntry(h, MigrateEntryDef, brokenKey, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding Key value 'not-a-hash'")
  })
}

func TestMigrateEntryFromJSON(t *testing.T) {
  Convey("MigrateEntry should be unserializable from JSON", t, func() {
    dnaHash, err := genTestStringHash()
    So(err, ShouldBeNil)

    key, err := genTestStringHash()
    So(err, ShouldBeNil)

    data, err := genTestString()
    So(err, ShouldBeNil)

    migrateType := randomSliceItem([]string{MigrateEntryTypeOpen, MigrateEntryTypeClose})

    jsonString := "{\"Type\":\"" + migrateType + "\",\"DNAHash\":\"" + dnaHash.String() + "\",\"Key\":\"" + key.String() + "\",\"Data\":\"" + data + "\"}"
    entry, err := MigrateEntryFromJSON(jsonString)
    So(err, ShouldBeNil)

    So(err, ShouldBeNil)
    So(entry.DNAHash, ShouldEqual, dnaHash)
    So(entry.Key, ShouldEqual, key)
    So(entry.Data, ShouldEqual, data)
    So(entry.Type, ShouldEqual, migrateType)
  })
}

func TestMigrateEntryToJSON(t *testing.T) {
  entry, err := genTestMigrateEntry()
  if err != nil {
    panic(err)
  }

  Convey("MigrateEntry should convert to JSON and roundtrip safely", t, func() {
    var j string
    var err error
		j, err = entry.ToJSON()

    if err != nil {
      panic(err)
    }

		So(err, ShouldBeNil)
		So(j, ShouldEqual, fmt.Sprintf(`{"Type":"open","DNAHash":"%s","Key":"%s","Data":"%s"}`, entry.DNAHash, entry.Key, entry.Data))

    roundtrip, err := MigrateEntryFromJSON(j)
    if err != nil {
      panic(err)
    }
    So(roundtrip, ShouldResemble, entry)
	})
}
