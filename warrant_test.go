package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"

	"testing"
)

func TestSelfRevocationWarrant(t *testing.T) {
	oldH, oldPrivKey := makePeer("peer1")
	newH, newPrivKey := makePeer("peer2")

	revocation, _ := NewSelfRevocation(oldPrivKey, newPrivKey, []byte("extra data"))

	w, err := NewSelfRevocationWarrant(revocation)

	Convey("NewSelfRevocation should create one", t, func() {
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", w.Revocation), ShouldEqual, fmt.Sprintf("%v", *revocation))
	})

	Convey("it should have a type", t, func() {
		So(w.Type(), ShouldEqual, SelfRevocationType)
	})

	Convey("it should have the two revocation parties", t, func() {
		parties, err := w.Parties()
		So(err, ShouldBeNil)
		So(len(parties), ShouldEqual, 2)
		So(peer.IDB58Encode(oldH), ShouldEqual, parties[0].String())
		So(peer.IDB58Encode(newH), ShouldEqual, parties[1].String())
	})

	Convey("it should have a payload property", t, func() {
		payload, err := w.Property("payload")
		So(err, ShouldBeNil)
		So(string(payload.([]byte)), ShouldEqual, "extra data")

		_, err = w.Property("foo")
		So(err, ShouldEqual, WarrantPropertyNotFoundErr)
	})

	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("verification should fail if not true in context", t, func() {
		err = w.Verify(h)
		So(err.Error(), ShouldEqual, "expected old key to be modified on DHT")
	})

	Convey("verification should succeed if true in context", t, func() {
		oldNodeIDStr := h.nodeIDStr
		_, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType,
			Code: fmt.Sprintf(`updateAgent({Revocation:"some revocation data"})`)})
		So(err, ShouldBeNil)
		header := h.chain.Top()
		entry, _, _ := h.chain.GetEntry(header.EntryLink)
		revocation := &SelfRevocation{}
		revocation.Unmarshal(entry.Content().(AgentEntry).Revocation)
		w, err := NewSelfRevocationWarrant(revocation)

		parties, err := w.Parties()
		So(oldNodeIDStr, ShouldEqual, parties[0].String())
		So(h.nodeIDStr, ShouldEqual, parties[1].String())

		err = w.Verify(h)
		So(err, ShouldBeNil)
	})

	Convey("it should encode and decode warrants", t, func() {
		encoded, err := w.Encode()
		So(err, ShouldBeNil)
		w1 := &SelfRevocationWarrant{}
		err = w1.Decode(encoded)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", w1), ShouldEqual, fmt.Sprintf("%v", w))

		w2, err := DecodeWarrant(SelfRevocationType, encoded)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", w2), ShouldEqual, fmt.Sprintf("%v", w))

		w2, err = DecodeWarrant(99, encoded)
		So(w2, ShouldBeNil)
		So(err, ShouldEqual, UnknownWarrantTypeErr)

	})
}
