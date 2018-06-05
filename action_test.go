package holochain

import (
	"fmt"
	. "github.com/holochain/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestValidateAction(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	var err error

	// these test the generic properties of ValidateAction using a commit action
	// as an example
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
		So(fmt.Sprintf("%v", d), ShouldEqual, "&{evenNumbers zygo public  <nil>}")
	})
	Convey("an invalid action returns the ValidationFailedErr", t, func() {
		entry := &GobEntry{C: "1"}
		a := NewCommitAction("evenNumbers", entry)
		_, err = h.ValidateAction(a, a.entryType, nil, []peer.ID{h.nodeID})
		So(IsValidationFailedErr(err), ShouldBeTrue)
	})

	// these test the sys type cases
	Convey("adding or changing dna should fail", t, func() {
		entry := &GobEntry{C: "fakeDNA"}
		a := NewCommitAction(DNAEntryType, entry)
		_, err = h.ValidateAction(a, a.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrNotValidForDNAType)
		ap := NewPutAction(DNAEntryType, entry, nil)
		_, err = h.ValidateAction(ap, ap.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrNotValidForDNAType)
		am := NewModAction(DNAEntryType, entry, HashFromPeerID(h.nodeID))
		_, err = h.ValidateAction(am, am.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrNotValidForDNAType)
	})

	Convey("modifying a headers entry should fail", t, func() {
		hd := h.Chain().Top()
		j, _ := hd.ToJSON()
		entryStr := fmt.Sprintf(`[{"Header":%s,"Role":"someRole","Source":"%s"}]`, j, h.nodeID.Pretty())
		am := NewModAction(HeadersEntryType, &GobEntry{C: entryStr}, HashFromPeerID(h.nodeID))
		_, err = h.ValidateAction(am, am.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrNotValidForHeadersType)
	})
}

func TestSysValidateEntry(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("key entry should be a public key", t, func() {
		e := &GobEntry{}
		err := sysValidateEntry(h, KeyEntryDef, e, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)
		e.C = []byte{1, 2, 3}
		err = sysValidateEntry(h, KeyEntryDef, e, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)

		e.C = "not b58 encoded public key!"
		err = sysValidateEntry(h, KeyEntryDef, e, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)

		pk, _ := h.agent.EncodePubKey()
		e.C = pk
		err = sysValidateEntry(h, KeyEntryDef, e, nil)
		So(err, ShouldBeNil)
	})

	Convey("an agent entry should have the correct structure as defined", t, func() {
		e := &GobEntry{}
		err := sysValidateEntry(h, AgentEntryDef, e, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)

		// bad agent entry (empty)
		e.C = ""
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)

		ae, _ := h.agent.AgentEntry(nil)
		// bad public key
		ae.PublicKey = ""
		a, _ := ae.ToJSON()
		e.C = a
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)

		ae, _ = h.agent.AgentEntry(nil)
		// bad public key
		ae.PublicKey = "not b58 encoded public key!"
		a, _ = ae.ToJSON()
		e.C = a
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)

		ae, _ = h.agent.AgentEntry(nil)
		// bad revocation
		ae.Revocation = string([]byte{1, 2, 3})
		a, _ = ae.ToJSON()
		e.C = a
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)

		ae, _ = h.agent.AgentEntry(nil)
		a, _ = ae.ToJSON()
		e.C = a
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(err, ShouldBeNil)
	})

	_, def, _ := h.GetEntryDef("rating")

	Convey("a nil entry is invalid", t, func() {
		err := sysValidateEntry(h, def, nil, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)
		So(err.Error(), ShouldEqual, "Validation Failed: nil entry invalid")
	})

	Convey("validate on a schema based entry should check entry against the schema", t, func() {
		profile := `{"firstName":"Eric"}` // missing required lastName
		_, def, _ := h.GetEntryDef("profile")

		err := sysValidateEntry(h, def, &GobEntry{C: profile}, nil)
		So(IsValidationFailedErr(err), ShouldBeTrue)
		So(err.Error(), ShouldEqual, "Validation Failed: validator profile failed: object property 'lastName' is required")
	})

	Convey("validate on a links entry should fail if not formatted correctly", t, func() {
		err := sysValidateEntry(h, def, &GobEntry{C: "badjson"}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry, invalid json: invalid character 'b' looking for beginning of value")

		err = sysValidateEntry(h, def, &GobEntry{C: `{}`}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: you must specify at least one link")

		err = sysValidateEntry(h, def, &GobEntry{C: `{"Links":[{}]}`}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: missing Base")

		err = sysValidateEntry(h, def, &GobEntry{C: `{"Links":[{"Base":"x","Link":"x","Tag":"sometag"}]}`}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: Base multihash too short. must be > 3 bytes")
		err = sysValidateEntry(h, def, &GobEntry{C: `{"Links":[{"Base":"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5","Link":"x","Tag":"sometag"}]}`}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: Link multihash too short. must be > 3 bytes")

		err = sysValidateEntry(h, def, &GobEntry{C: `{"Links":[{"Base":"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5","Link":"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5"}]}`}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "invalid links entry: missing Tag")
	})

	Convey("validate headers entry should fail if it doesn't match the headers entry schema", t, func() {
		err := sysValidateEntry(h, HeadersEntryDef, &GobEntry{C: ""}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "unexpected end of JSON input")

		err = sysValidateEntry(h, HeadersEntryDef, &GobEntry{C: `{"Fish":2}`}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Validation Failed: validator %header failed: value must be a slice (was: map[string]interface {})")

	})

	Convey("validate headers entry should succeed on valid entry", t, func() {
		hd := h.Chain().Top()
		j, _ := hd.ToJSON()
		entryStr := fmt.Sprintf(`[{"Header":%s,"Role":"someRole","Source":"%s"}]`, j, h.nodeID.Pretty())
		err := sysValidateEntry(h, HeadersEntryDef, &GobEntry{C: entryStr}, nil)
		So(err, ShouldBeNil)
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
