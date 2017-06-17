package holochain

import (
	// "fmt"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestValidateAction(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	var err error

	// these test the generic properties of ValidateAction using a commit action as an example
	Convey("it should fail if a validator doesn't exist for the entry type", t, func() {
		entry := &GobEntry{C: "foo"}
		a := NewCommitAction("bogusType", entry)
		_, err = h.ValidateAction(a, a.entryType, nil, []peer.ID{h.nodeID})
		So(err.Error(), ShouldEqual, "no definition for entry type: bogusType")
	})

	Convey("a valid entry returns the entry def", t, func() {
		entry := &GobEntry{C: "2"}
		a := NewCommitAction("evenNumbers", entry)
		var d *EntryDef
		d, err = h.ValidateAction(a, a.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", d), ShouldEqual, "&{evenNumbers zygo   public <nil>}")
	})
	Convey("an invalid action returns the ValidationFailedErr", t, func() {
		entry := &GobEntry{C: "1"}
		a := NewCommitAction("evenNumbers", entry)
		_, err = h.ValidateAction(a, a.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ValidationFailedErr)
	})
}

func TestSysValidateEntry(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("a nil entry is invalid", t, func() {
		err := sysValidateEntry(h, nil, nil)
		So(err.Error(), ShouldEqual, "nil entry invalid")
	})

	Convey("validate on a schema based entry should check entry against the schema", t, func() {
		profile := `{"firstName":"Eric"}` // missing required lastName
		_, def, _ := h.GetEntryDef("profile")

		err := sysValidateEntry(h, def, &GobEntry{C: profile})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "validator profile.json failed: object property 'lastName' is required")
	})

	_, def, _ := h.GetEntryDef("rating")

	Convey("validate on a links entry should fail if not formatted correctly", t, func() {
		err := sysValidateEntry(h, def, &GobEntry{C: "badjson"})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry, invalid json: invalid character 'b' looking for beginning of value")

		err = sysValidateEntry(h, def, &GobEntry{C: `{}`})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: you must specify at least one link")

		err = sysValidateEntry(h, def, &GobEntry{C: `{"Links":[{}]}`})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: missing Base")

		err = sysValidateEntry(h, def, &GobEntry{C: `{"Links":[{"Base":"x","Link":"x","Tag":"sometag"}]}`})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: Base multihash too short. must be > 3 bytes")
		err = sysValidateEntry(h, def, &GobEntry{C: `{"Links":[{"Base":"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5","Link":"x","Tag":"sometag"}]}`})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: Link multihash too short. must be > 3 bytes")

		err = sysValidateEntry(h, def, &GobEntry{C: `{"Links":[{"Base":"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5","Link":"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5"}]}`})
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: missing Tag")
	})
}

func TestSysValidateMod(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	hash := commit(h, "evenNumbers", "2")
	_, def, _ := h.GetEntryDef("evenNumbers")

	Convey("it should check that entry types match on mod", t, func() {
		a := NewModAction("oddNumbers", &GobEntry{}, hash)
		err := a.SysValidation(h, def, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrEntryTypeMismatch)
	})

	Convey("it should check that entry isn't linking ", t, func() {
		a := NewModAction("rating", &GobEntry{}, hash)
		_, ratingsDef, _ := h.GetEntryDef("rating")
		err := a.SysValidation(h, ratingsDef, []peer.ID{h.nodeID})
		So(err.Error(), ShouldEqual, "Can't mod Links entry")
	})

	Convey("it should check that entry validates", t, func() {
		a := NewModAction("evenNumbers", nil, hash)
		err := a.SysValidation(h, def, []peer.ID{h.nodeID})
		So(err.Error(), ShouldEqual, "nil entry invalid")
	})
}

func TestSysValidateDel(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	hash := commit(h, "evenNumbers", "2")
	_, def, _ := h.GetEntryDef("evenNumbers")

	Convey("it should check that entry types match on del", t, func() {
		a := NewDelAction("oddNumbers", DelEntry{Hash: hash})
		err := a.SysValidation(h, def, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrEntryTypeMismatch)
	})

	Convey("it should check that entry isn't linking ", t, func() {
		a := NewDelAction("rating", DelEntry{Hash: hash})
		_, ratingsDef, _ := h.GetEntryDef("rating")
		err := a.SysValidation(h, ratingsDef, []peer.ID{h.nodeID})
		So(err.Error(), ShouldEqual, "Can't del Links entry")
	})
}

func TestCheckArgCount(t *testing.T) {
	Convey("it should check for wrong number of args", t, func() {
		args := []Arg{{}}
		err := checkArgCount(args, 2)
		So(err, ShouldEqual, ErrWrongNargs)

		// test with args that are optional: two that are required and one not
		args = []Arg{{}, {}, {Optional: true}}
		err = checkArgCount(args, 1)
		So(err, ShouldEqual, ErrWrongNargs)

		err = checkArgCount(args, 2)
		So(err, ShouldBeNil)

		err = checkArgCount(args, 3)
		So(err, ShouldBeNil)

		err = checkArgCount(args, 4)
		So(err, ShouldEqual, ErrWrongNargs)
	})
}

func TestActionGetLocal(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	hash := commit(h, "secret", "31415")

	Convey("non local get should fail for private entries", t, func() {
		req := GetReq{H: hash, GetMask: GetMaskEntry}
		_, err := NewGetAction(req, &GetOptions{GetMask: req.GetMask}).Do(h)
		So(err.Error(), ShouldEqual, "hash not found")
	})

	Convey("it should fail to get non-existent private local values", t, func() {
		badHash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		req := GetReq{H: badHash, GetMask: GetMaskEntry}
		_, err := NewGetAction(req, &GetOptions{GetMask: req.GetMask, Local: true}).Do(h)
		So(err.Error(), ShouldEqual, "hash not found")
	})

	Convey("it should get private local values", t, func() {
		req := GetReq{H: hash, GetMask: GetMaskEntry}
		rsp, err := NewGetAction(req, &GetOptions{GetMask: req.GetMask, Local: true}).Do(h)
		So(err, ShouldBeNil)
		getResp := rsp.(GetResp)
		So(getResp.Entry.Content().(string), ShouldEqual, "31415")
	})
}
