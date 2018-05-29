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
	mt := setupMultiNodeTesting(3)
	defer mt.cleanupMultiNodeTesting()
	h1 := mt.nodes[0]

	Convey("ActionMigrate should share as a PUT on the DHT and roundtrip as JSON", t, func() {
		header, err := genTestHeader()
		if err != nil {
			panic(err)
		}
		action := ActionMigrate{header: header}

		// Can share from some node
		err = action.Share(h1, action.entry.Def())
		So(err, ShouldBeNil)

		entryJSON, _ := action.entry.ToJSON()
		// Can get the PUT MigrateEntry from the same node
		roundtrip, _, _, _, err := h1.dht.Get(action.header.EntryLink, StatusAny, GetMaskAll)
		So(err, ShouldBeNil)
		So(entryJSON, ShouldEqual, string(roundtrip))

		// Can get the PUT MigrateEntry from a different node
		// @TODO does not work...
		// h2 := mt.nodes[2]
		// roundtrip2, _, _, _, err := h2.dht.Get(action.header.EntryLink, StatusAny, GetMaskAll)
		// So(err, ShouldBeNil)
		// So(entryJSON, ShouldEqual, string(roundtrip2))
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

	Convey("ActionMigrate SysValidation should return an ErrActionMissingHeader error if header is missing", t, func() {
		action := ActionMigrate{}
		err := action.SysValidation(h, action.entry.Def(), nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrActionMissingHeader)
	})

	Convey("ActionMigrate SysValidation should validate the entry", t, func() {
		header, err := genTestHeader()
		if err != nil {
			panic(err)
		}
		action := ActionMigrate{header: header}
		err = action.SysValidation(h, action.entry.Def(), nil, []peer.ID{h.nodeID})
		// the entry is empty so there should be validation complaints
		So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding Chain value ''")

		action.entry, err = genTestMigrateEntry()
		if err != nil {
			panic(err)
		}
		err = action.SysValidation(h, action.entry.Def(), nil, []peer.ID{h.nodeID})
		So(err, ShouldBeNil)
	})
}

func TestMigrateCheckValidationRequest(t *testing.T) {
	Convey("MigrateAction CheckValidationRequest should always pass", t, func() {
		action := ActionMigrate{}
		So(action.CheckValidationRequest(action.entry.Def()), ShouldBeNil)
	})
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
