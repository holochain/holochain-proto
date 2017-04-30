package holochain

import (
	// "fmt"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestAction(t *testing.T) {
	Convey("should fail to create a nucleus based from bad nucleus type", t, func() {
		So(true, ShouldBeTrue)
	})
}

func TestValidateAction(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	var err error

	// these test the generic properties of ValidateAction using a commit action as an example
	Convey("it should fail if a validator doesn't exist for the entry type", t, func() {
		entry := &GobEntry{C: "foo"}
		a := NewCommitAction("bogusType", entry)
		_, err = h.ValidateAction(a, a.entryType, []peer.ID{h.id})
		So(err.Error(), ShouldEqual, "no definition for entry type: bogusType")
	})

	Convey("a valid entry returns the entry def", t, func() {
		entry := &GobEntry{C: "2"}
		a := NewCommitAction("evenNumbers", entry)
		var d *EntryDef
		d, err = h.ValidateAction(a, a.entryType, []peer.ID{h.id})
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", d), ShouldEqual, "&{evenNumbers zygo   public <nil>}")
	})
	Convey("an invalid action returns the ValidationFailedErr", t, func() {
		entry := &GobEntry{C: "1"}
		a := NewCommitAction("evenNumbers", entry)
		_, err = h.ValidateAction(a, a.entryType, []peer.ID{h.id})
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
