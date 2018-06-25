package holochain

import (
	"bytes"
	"crypto/rand"
	"fmt"
	. "github.com/HC-Interns/holochain-proto/hash"
	ic "github.com/libp2p/go-libp2p-crypto"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestHeaderNew(t *testing.T) {
	h, key, now := chainTestSetup()

	Convey("it should make a header and return its hash", t, func() {
		e := GobEntry{C: "some data"}
		ph := NullHash()
		hash, header, err := newHeader(h, now, "evenNumbers", &e, key, ph, ph, NullHash())

		So(err, ShouldBeNil)
		// encode the header and create a hash of it
		b, _ := header.Marshal()
		var h2 Hash
		h2, _ = Sum(h, b)
		So(h2.String(), ShouldEqual, hash.String())
	})

	Convey("it should make a header and return its hash if change header", t, func() {
		e := GobEntry{C: "some data"}
		ph := NullHash()
		modHash, _ := NewHash("QmP1DfoUjiWH2ZBo1PBH6FupdBucbDepx3HpWmEY6JMUpY")
		hash, header, err := newHeader(h, now, "evenNumbers", &e, key, ph, ph, modHash)

		So(err, ShouldBeNil)
		// encode the header and create a hash of it
		b, _ := header.Marshal()
		var h2 Hash
		h2, _ = Sum(h, b)
		So(h2.String(), ShouldEqual, hash.String())
	})
}

func TestHeaderToJSON(t *testing.T) {
	h, key, now := chainTestSetup()
	Convey("it should convert a header to JSON", t, func() {
		e := GobEntry{C: "1234"}
		hd := testHeader(h, "evenNumbers", &e, key, now)
		j, err := hd.ToJSON()
		So(err, ShouldBeNil)
		So(j, ShouldStartWith, `{"Type":"evenNumbers","Time":"`)
		So(j, ShouldEndWith, `","EntryLink":"QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuU2","HeaderLink":"QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuUi","TypeLink":"","Signature":"3eDinUfqsX4V2iuwFvFNSwyy4KEugYj6DPpssjrAsabkVvozBrWrLJRuA9AXhiN8R3MzZvyLfW2BV8zKDevSDiVR"}`)
	})
}

func TestHeaderMarshal(t *testing.T) {
	h, key, now := chainTestSetup()

	e := GobEntry{C: "some  data"}
	hd := testHeader(h, "evenNumbers", &e, key, now)
	hd.Change, _ = NewHash("QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuU2")
	Convey("it should round-trip", t, func() {
		b, err := hd.Marshal()
		So(err, ShouldBeNil)
		var nh Header
		err = (&nh).Unmarshal(b, 34)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", nh), ShouldEqual, fmt.Sprintf("%v", *hd))
	})
}

func TestSignatureB58(t *testing.T) {
	h, key, now := chainTestSetup()
	e := GobEntry{C: "1234"}
	hd := testHeader(h, "evenNumbers", &e, key, now)
	Convey("it should round-trip", t, func() {
		b58sig := hd.Sig.B58String()
		So(b58sig, ShouldEqual, "3eDinUfqsX4V2iuwFvFNSwyy4KEugYj6DPpssjrAsabkVvozBrWrLJRuA9AXhiN8R3MzZvyLfW2BV8zKDevSDiVR")
		newSig := SignatureFromB58String(b58sig)
		So(newSig.Equal(hd.Sig), ShouldBeTrue)
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
	sig, err := key.Sign([]byte(hd.EntryLink))
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
	h1.Change = NullHash()

	//h1.Sig.S.321)
	return h1
}
