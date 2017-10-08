package holochain

import (
	// "fmt"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestValidateAction(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
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
		So(fmt.Sprintf("%v", d), ShouldEqual, "&{evenNumbers zygo public  <nil>}")
	})
	Convey("an invalid action returns the ValidationFailedErr", t, func() {
		entry := &GobEntry{C: "1"}
		a := NewCommitAction("evenNumbers", entry)
		_, err = h.ValidateAction(a, a.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ValidationFailedErr)
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

	Convey("deleting all sys entry types should fail", t, func() {
		a := NewDelAction(DNAEntryType, DelEntry{})
		_, err = h.ValidateAction(a, a.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrNotValidForDNAType)
		a.entryType = KeyEntryType
		_, err = h.ValidateAction(a, a.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrNotValidForKeyType)
		a.entryType = AgentEntryType
		_, err = h.ValidateAction(a, a.entryType, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrNotValidForAgentType)
	})
}

func TestSysValidateEntry(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("key entry should be a public key", t, func() {
		e := &GobEntry{}
		err := sysValidateEntry(h, KeyEntryDef, e, nil)
		So(err, ShouldEqual, ValidationFailedErr)
		e.C = []byte{1, 2, 3}
		err = sysValidateEntry(h, KeyEntryDef, e, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		e.C = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6}
		err = sysValidateEntry(h, KeyEntryDef, e, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		pk, _ := ic.MarshalPublicKey(h.agent.PubKey())
		e.C = pk
		err = sysValidateEntry(h, KeyEntryDef, e, nil)
		So(err, ShouldBeNil)
	})

	Convey("an agent entry should have the correct structure as defined", t, func() {
		e := &GobEntry{}
		err := sysValidateEntry(h, AgentEntryDef, e, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		// bad agent entry (empty)
		e.C = AgentEntry{}
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		ae, _ := h.agent.AgentEntry(nil)
		// bad public key
		ae.PublicKey = nil
		e.C = ae
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		ae, _ = h.agent.AgentEntry(nil)
		// bad public key
		ae.PublicKey = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6}
		e.C = ae
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		ae, _ = h.agent.AgentEntry(nil)
		// bad revocation
		ae.Revocation = []byte{1, 2, 3}
		e.C = ae
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		ae, _ = h.agent.AgentEntry(nil)
		e.C = ae
		err = sysValidateEntry(h, AgentEntryDef, e, nil)
		So(err, ShouldBeNil)
	})

	_, def, _ := h.GetEntryDef("rating")

	Convey("a nil entry is invalid", t, func() {
		err := sysValidateEntry(h, def, nil, nil)
		So(err.Error(), ShouldEqual, "nil entry invalid")
	})

	Convey("validate on a schema based entry should check entry against the schema", t, func() {
		profile := `{"firstName":"Eric"}` // missing required lastName
		_, def, _ := h.GetEntryDef("profile")

		err := sysValidateEntry(h, def, &GobEntry{C: profile}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "validator profile failed: object property 'lastName' is required")
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
}

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
		So(err.Error(), ShouldEqual, "Can't mod Links entry")
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
		So(err.Error(), ShouldEqual, "mod: missing header")
	})

	Convey("it should check that replaces is doesn't make a loop", t, func() {
		a := NewModAction("evenNumbers", &GobEntry{}, hash)
		a.header = &Header{EntryLink: hash}
		err := a.SysValidation(h, def, nil, []peer.ID{h.nodeID})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "mod: replaces must be different from original hash")
	})

}

func TestSysValidateDel(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	hash := commit(h, "evenNumbers", "2")
	_, def, _ := h.GetEntryDef("evenNumbers")

	Convey("it should check that entry types match on del", t, func() {
		a := NewDelAction("oddNumbers", DelEntry{Hash: hash})
		err := a.SysValidation(h, def, nil, []peer.ID{h.nodeID})
		So(err, ShouldEqual, ErrEntryTypeMismatch)
	})

	Convey("it should check that entry isn't linking ", t, func() {
		a := NewDelAction("rating", DelEntry{Hash: hash})
		_, ratingsDef, _ := h.GetEntryDef("rating")
		err := a.SysValidation(h, ratingsDef, nil, []peer.ID{h.nodeID})
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

func TestActionGet(t *testing.T) {
	nodesCount := 3
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()

	h := mt.nodes[0]

	e := GobEntry{C: "3"}
	hash, _ := e.Sum(h.hashSpec)

	Convey("receive should return not found if it doesn't exist", t, func() {
		m := h.node.NewMessage(GET_REQUEST, GetReq{H: hash})
		_, err := ActionReceiver(h, m)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	commit(h, "oddNumbers", "3")
	m := h.node.NewMessage(GET_REQUEST, GetReq{H: hash})
	Convey("receive should return value if it exists", t, func() {
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", e))
	})

	ringConnect(t, mt.ctx, mt.nodes, nodesCount)
	Convey("receive should return closer peers if it can", t, func() {
		h2 := mt.nodes[2]
		r, err := ActionReceiver(h2, m)
		So(err, ShouldBeNil)
		resp := r.(CloserPeersResp)
		So(len(resp.CloserPeers), ShouldEqual, 1)
		So(peer.ID(resp.CloserPeers[0].ID).Pretty(), ShouldEqual, "QmUfY4WeqD3UUfczjdkoFQGEgCAVNf7rgFfjdeTbr7JF1C")
	})
}

func TestActionGetLocal(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

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
