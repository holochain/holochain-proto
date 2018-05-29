package holochain

import (
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

// ActionMigrate

func TestMigrateName(t *testing.T) {
	Convey("migrate action should have the right name", t, func() {
		a := ActionMigrate{}
		So(a.Name(), ShouldEqual, "migrate")
	})
}

func TestMigrateEntry(t *testing.T) {
	Convey("empty migrate action Entry() should be retreive a serialized JSON of an empty entry in a GobEntry", t, func() {
		action := ActionMigrate{}
		So(action.Entry(), ShouldResemble, &GobEntry{C: "{\"Type\":\"\",\"Chain\":\"\",\"User\":\"\",\"Data\":\"\"}"})
	})

	Convey("entries with vals work with Entry()", t, func() {
		chain, err := genTestStringHash()
		if err != nil {
			panic(err)
		}
		user, err := genTestStringHash()
		if err != nil {
			panic(err)
		}
		entry := MigrateEntry{Chain: chain, User: user}
		action := ActionMigrate{entry: entry}

		So(action.Entry(), ShouldResemble, &GobEntry{C: "{\"Type\":\"\",\"Chain\":\"" + chain.String() + "\",\"User\":\"" + user.String() + "\",\"Data\":\"\"}"})
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

	Convey("migrate action should be able to set and get header", t, func() {
		action := ActionMigrate{}
		header, err := genTestHeader()
		if err != nil {
			panic(err)
		}
		So(action.GetHeader(), ShouldEqual, nil)
		action.SetHeader(header)
		So(action.GetHeader(), ShouldEqual, header)
		action.SetHeader(nil)
		So(action.GetHeader(), ShouldEqual, nil)
	})
}

func TestMigrateShare(t *testing.T) {
	mt := setupMultiNodeTesting(1)
	defer mt.cleanupMultiNodeTesting()
	h := mt.nodes[0]

	Convey("ActionMigrate should return an ErrActionMissingHeader error if header is missing", t, func() {
		action := ActionMigrate{}
		err := action.Share(h, action.entry.Def())
		So(err, ShouldEqual, ErrActionMissingHeader)
	})

	Convey("ActionMigrate should share as a PUT on the DHT and roundtrip as JSON", t, func() {
		header, err := genTestHeader()
		if err != nil {
			panic(err)
		}
		action := ActionMigrate{header: header}

		err = action.Share(h, action.entry.Def())
		So(err, ShouldBeNil)

		entryJSON, _ := action.entry.ToJSON()
		roundtrip, _, _, _, err := h.dht.Get(action.header.EntryLink, StatusAny, GetMaskAll)

		So(err, ShouldBeNil)
		So(entryJSON, ShouldEqual, string(roundtrip))
	})
}

func TestMigrateActionSysValidation(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("it should invalidate DNAEntryDef", t, func() {
		action := ActionMigrate{}
		err := action.SysValidation(h, DNAEntryDef, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrEntryDefInvalid)
	})
}

func TestMigrateCheckValidationRequest(t *testing.T) {
	// @TODO
}

func TestMigrateReceive(t *testing.T) {
	// @TODO
}

// APIFnMigrate

func TestAPIFnMigrateName(t *testing.T) {
	Convey("migrate action function should have the right name", t, func() {
		fn := &APIFnMigrate{}
		So(fn.Name(), ShouldEqual, "migrate")
	})
}

func TestAPIFnMigrateArgs(t *testing.T) {
	Convey("APIFnMigrate should have the correct args", t, func() {
		fn := &APIFnMigrate{}
		So(fn.Args(), ShouldResemble, []Arg{{Name: "migrationType", Type: StringArg}, {Name: "DNA", Type: HashArg}, {Name: "ID", Type: HashArg}, {Name: "data", Type: StringArg}})
	})
}

func TestAPIFnMigrateCall(t *testing.T) {
	// @TODO
}
