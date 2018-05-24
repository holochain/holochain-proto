package holochain

import (
	. "github.com/smartystreets/goconvey/convey"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"testing"
)

func TestSysValidateMod(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	hash := commit(h, "evenNumbers", "2")
	_, def, _ := h.GetEntryDef("evenNumbers")

	/* This is actually bogus because it assumes we have the entry type in our chain but
	           might be in a different chain.
		Convey("it should check that entry types match on mod", t, func() {
			a := NewModAction("oddNumbers", &GobEntry{}, hash)
			err := a.SysValidation(h, def, nil, []peer.ID{h.nodeID})
			So(err, ShouldEqual, ErrEntryTypeMismatch)
		})
	*/

	Convey("it should check that entry isn't linking ", t, func() {
		a := NewModAction("rating", &GobEntry{}, hash)
		_, ratingsDef, _ := h.GetEntryDef("rating")
		err := a.SysValidation(h, ratingsDef, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrModInvalidForLinks)
	})

	Convey("it should check that entry validates", t, func() {
		a := NewModAction("evenNumbers", nil, hash)
		err := a.SysValidation(h, def, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrNilEntryInvalid)
	})

	Convey("it should check that header isn't missing", t, func() {
		a := NewModAction("evenNumbers", &GobEntry{}, hash)
		err := a.SysValidation(h, def, nil, []peer.ID{h.nodeID})
		So(err, ShouldBeError)
		So(err, ShouldEqual, ErrModMissingHeader)
	})

	Convey("it should check that replaces is doesn't make a loop", t, func() {
		a := NewModAction("evenNumbers", &GobEntry{}, hash)
		a.header = &Header{EntryLink: hash}
		err := a.SysValidation(h, def, nil, []peer.ID{h.nodeID})
		So(err, ShouldBeError)
		So(err, ShouldEqual, ErrModReplacesHashNotDifferent)
	})

}
