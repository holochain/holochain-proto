package holochain

import (
	"fmt"
	. "github.com/HC-Interns/holochain-proto/hash"
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
		So(action.Entry(), ShouldResemble, &GobEntry{C: "{\"Type\":\"\",\"DNAHash\":\"\",\"Key\":\"\",\"Data\":\"\"}"})
	})

	Convey("entries with vals work with Entry()", t, func() {
		dnaHash, err := genTestStringHash()
		So(err, ShouldBeNil)

		key, err := genTestStringHash()
		So(err, ShouldBeNil)

		entry := MigrateEntry{DNAHash: dnaHash, Key: key}
		action := ActionMigrate{entry: entry}

		So(action.Entry(), ShouldResemble, &GobEntry{C: "{\"Type\":\"\",\"DNAHash\":\"" + dnaHash.String() + "\",\"Key\":\"" + key.String() + "\",\"Data\":\"\"}"})
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
		So(err, ShouldBeNil)

		So(action.GetHeader(), ShouldEqual, nil)
		action.SetHeader(header)
		So(action.GetHeader(), ShouldEqual, header)
		action.SetHeader(nil)
		So(action.GetHeader(), ShouldEqual, nil)
	})
}

func TestMigrateCallShare(t *testing.T) {
	n := 3
	mt := setupMultiNodeTesting(n)
	ringConnect(t, mt.ctx, mt.nodes, n)
	defer mt.cleanupMultiNodeTesting()

	Convey("ActionMigrate should share as a PUT on the DHT and roundtrip as JSON", t, func() {
		var err error
		header, err := genTestHeader()
		entry, err := genTestMigrateEntry()
		So(err, ShouldBeNil)

		action := ActionMigrate{header: header, entry: entry}

		// Can share from some node
		var dhtHash Hash
		fn := &APIFnMigrate{action: action}
		callResponse, err := fn.Call(mt.nodes[0])
		dhtHash, ok := callResponse.(Hash)
		So(ok, ShouldBeTrue)
		So(err, ShouldBeNil)

		// Can get the PUT MigrateEntry from any node
		for i := 0; i < n; i++ {
			fmt.Printf("\nTesting retrieval of MigrateEntry PUT from node %d\n", i)

			request := GetReq{H: dhtHash, StatusMask: StatusLive, GetMask: GetMaskEntry}
			response, err := callGet(mt.nodes[i], request, &GetOptions{GetMask: request.GetMask})
			r, ok := response.(GetResp)

			So(ok, ShouldBeTrue)
			So(err, ShouldBeNil)

			So(&r.Entry, ShouldResemble, action.Entry())
		}
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
		So(err, ShouldBeNil)

		action := ActionMigrate{header: header}
		err = action.SysValidation(h, action.entry.Def(), nil, []peer.ID{h.nodeID})
		// the entry is empty so there should be validation complaints
		So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding DNAHash value ''")

		action.entry, err = genTestMigrateEntry()
		So(err, ShouldBeNil)

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
	mt := setupMultiNodeTesting(1)
	defer mt.cleanupMultiNodeTesting()
	h := mt.nodes[0]

	Convey("MigrateAction Receive is always an error", t, func() {
		action := ActionMigrate{}
		msg := h.node.NewMessage(PUT_REQUEST, HoldReq{})
		response, err := action.Receive(h.dht, msg)
		So(err.Error(), ShouldEqual, "Action receive is invalid")
		So(response, ShouldBeNil)
	})
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
		expected := []Arg{{Name: "migrationType",
			Type: StringArg},
			{Name: "DNAHash",
				Type: HashArg},
			{Name: "Key",
				Type: HashArg},
			{Name: "data",
				Type: StringArg}}
		So(fn.Args(), ShouldResemble, expected)
	})
}
