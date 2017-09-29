package holochain

import (
	"bytes"
	"crypto/rand"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestNewHeader(t *testing.T) {
	h, key, now := chainTestSetup()

	Convey("it should make a header and return its hash", t, func() {
		e := GobEntry{C: "some data"}
		ph := NullHash()
		hash, header, err := newHeader(h, now, "evenNumbers", &e, key, ph, ph, nil)

		So(err, ShouldBeNil)
		// encode the header and create a hash of it
		b, _ := header.Marshal()
		var h2 Hash
		h2.Sum(h, b)
		So(h2.String(), ShouldEqual, hash.String())
	})

	Convey("it should make a header and return its hash if change header", t, func() {
		e := GobEntry{C: "some data"}
		ph := NullHash()
		delHash, _ := NewHash("QmP1DfoUjiWH2ZBo1PBH6FupdBucbDepx3HpWmEY6JMUpY")
		hash, header, err := newHeader(h, now, "evenNumbers", &e, key, ph, ph, &StatusChange{Action: DelAction, Hash: delHash})

		So(err, ShouldBeNil)
		// encode the header and create a hash of it
		b, _ := header.Marshal()
		var h2 Hash
		h2.Sum(h, b)
		So(h2.String(), ShouldEqual, hash.String())
	})
}

func TestMarshalHeader(t *testing.T) {
	h, key, now := chainTestSetup()

	e := GobEntry{C: "some  data"}
	hd := testHeader(h, "evenNumbers", &e, key, now)
	hd.Change.Action = ModAction
	Convey("it should round-trip", t, func() {
		b, err := hd.Marshal()
		So(err, ShouldBeNil)
		var nh Header
		err = (&nh).Unmarshal(b, 34)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", nh), ShouldEqual, fmt.Sprintf("%v", *hd))
	})
}

func TestMarshalSignature(t *testing.T) {
	var s Signature
	Convey("it should round-trip an empty signature", t, func() {
		var b bytes.Buffer

		err := MarshalSignature(&b, &s)
		So(err, ShouldBeNil)
		var ns Signature
		err = UnmarshalSignature(&b, &ns)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", ns), ShouldEqual, fmt.Sprintf("%v", s))
	})

	Convey("it should round-trip a random signature", t, func() {
		var b bytes.Buffer

		r := make([]byte, 64)
		_, err := rand.Read(r)
		s.S = r
		err = MarshalSignature(&b, &s)
		So(err, ShouldBeNil)
		var ns Signature
		err = UnmarshalSignature(&b, &ns)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", ns), ShouldEqual, fmt.Sprintf("%v", s))
	})
}

//----- test util functions

func testHeader(h HashSpec, t string, entry Entry, key ic.PrivKey, now time.Time) *Header {
	hd := mkTestHeader(t)
	sig, err := key.Sign(hd.EntryLink.H)
	if err != nil {
		panic(err)
	}
	hd.Sig = Signature{S: sig}
	return &hd
}

func mkTestHeader(t string) Header {
	hl, _ := NewHash("QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuUi")
	el, _ := NewHash("QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuU2")
	now := time.Unix(1, 1)           // pick a constant time so the test will always work
	h1 := Header{Time: now, Type: t, // Meta: "dog",
		HeaderLink: hl,
		EntryLink:  el,
		TypeLink:   NullHash(),
	}
	h1.Change.Hash = NullHash()

	//h1.Sig.S.321)
	return h1
}
