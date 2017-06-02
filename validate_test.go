package holochain

import (
	"bytes"
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"strings"
	"testing"
	"time"
)

func TestValidateReceiver(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("VALIDATE_PUT_REQUEST should fail if  body isn't a ValidateQuery", t, func() {
		m := h.node.NewMessage(VALIDATE_PUT_REQUEST, "fish")
		_, err := ValidateReceiver(h, m)
		So(err.Error(), ShouldEqual, "expected ValidateQuery got string")
	})
	Convey("VALIDATE_PUT_REQUEST should fail if hash doesn't exist", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		m := h.node.NewMessage(VALIDATE_PUT_REQUEST, ValidateQuery{H: hash})
		_, err := ValidateReceiver(h, m)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "hash not found")
	})
	Convey("VALIDATE_PUT_REQUEST should return entry by hash", t, func() {
		entry := GobEntry{C: "bogus entry data"}
		_, hd, err := h.NewEntry(time.Now(), "evenNumbers", &entry)

		m := h.node.NewMessage(VALIDATE_PUT_REQUEST, ValidateQuery{H: hd.EntryLink})
		r, err := ValidateReceiver(h, m)
		So(err, ShouldBeNil)
		vr := r.(ValidateResponse)
		So(vr.Type, ShouldEqual, "evenNumbers")
		So(fmt.Sprintf("%v", vr.Entry), ShouldEqual, fmt.Sprintf("%v", entry))
		So(fmt.Sprintf("%v", vr.Header), ShouldEqual, fmt.Sprintf("%v", *hd))
	})
	Convey("VALIDATE_LINK_REQUEST should fail if  body isn't a ValidateQuery", t, func() {
		m := h.node.NewMessage(VALIDATE_LINK_REQUEST, "fish")
		_, err := ValidateReceiver(h, m)
		So(err.Error(), ShouldEqual, "expected ValidateQuery got string")
	})
	Convey("VALIDATE_LINK_REQUEST should fail if hash doesn't exist", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		m := h.node.NewMessage(VALIDATE_LINK_REQUEST, ValidateQuery{H: hash})
		_, err := ValidateReceiver(h, m)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "hash not found")
	})

	entry := GobEntry{C: "bogus entry data"}
	_, hd, _ := h.NewEntry(time.Now(), "evenNumbers", &entry)
	hash := hd.EntryLink

	Convey("VALIDATE_LINK_REQUEST should return error if hash isn't a linking entry", t, func() {
		m := h.node.NewMessage(VALIDATE_LINK_REQUEST, ValidateQuery{H: hash})
		_, err := ValidateReceiver(h, m)
		So(err.Error(), ShouldEqual, "hash not of a linking entry")
	})

	Convey("VALIDATE_LINK_REQUEST should return entry by linking entry hash", t, func() {
		someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
		e := GobEntry{C: someData}
		_, phd, _ := h.NewEntry(time.Now(), "profile", &e)
		profileHash := phd.EntryLink
		e = GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String())}
		_, le, _ := h.NewEntry(time.Now(), "rating", &e)

		m := h.node.NewMessage(VALIDATE_LINK_REQUEST, ValidateQuery{H: le.EntryLink})
		r, err := ValidateReceiver(h, m)
		So(err, ShouldBeNil)
		vr := r.(ValidateResponse)
		So(vr.Type, ShouldEqual, "rating")
		So(fmt.Sprintf("%v", vr.Entry), ShouldEqual, fmt.Sprintf("%v", e))
		So(fmt.Sprintf("%v", vr.Header), ShouldEqual, fmt.Sprintf("%v", *le))
	})
}

func TestMakePackage(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("it should be able to make a full chain package", t, func() {
		req := PackagingReq{PkgReqChain: int64(PkgReqChainOptFull)}
		pkg, err := MakePackage(h, req)
		So(err, ShouldBeNil)

		var b bytes.Buffer
		h.chain.MarshalChain(&b, ChainMarshalFlagsOmitDNA)
		So(string(pkg.Chain), ShouldEqual, string(b.Bytes()))
	})
	Convey("it should be able to make a headers only chain package", t, func() {
		req := PackagingReq{PkgReqChain: int64(PkgReqChainOptHeaders)}
		pkg, err := MakePackage(h, req)
		So(err, ShouldBeNil)

		var b bytes.Buffer
		h.chain.MarshalChain(&b, ChainMarshalFlagsNoEntries+ChainMarshalFlagsOmitDNA)
		So(string(pkg.Chain), ShouldEqual, string(b.Bytes()))
	})
	Convey("it should be able to make an entries only chain package", t, func() {
		req := PackagingReq{PkgReqChain: int64(PkgReqChainOptEntries)}
		pkg, err := MakePackage(h, req)
		So(err, ShouldBeNil)

		var b bytes.Buffer
		h.chain.MarshalChain(&b, ChainMarshalFlagsNoHeaders+ChainMarshalFlagsOmitDNA)
		So(string(pkg.Chain), ShouldEqual, string(b.Bytes()))
	})

	Convey("it should be able to make package of a chain of just a few types", t, func() {
		entry := GobEntry{C: "2"}
		h.NewEntry(time.Now(), "evenNumbers", &entry)
		entry = GobEntry{C: "3"}
		h.NewEntry(time.Now(), "oddNumbers", &entry)

		req := PackagingReq{PkgReqChain: int64(PkgReqChainOptFull), PkgReqEntryTypes: []string{"oddNumbers"}}
		pkg, err := MakePackage(h, req)
		So(err, ShouldBeNil)

		var b bytes.Buffer
		h.chain.MarshalChain(&b, ChainMarshalFlagsOmitDNA, "oddNumbers")
		So(string(pkg.Chain), ShouldEqual, string(b.Bytes()))
	})

}

func TestGetValidationResponse(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	Convey("Sys defined entry types should return empty packages", t, func() {
		entry, _, err := h.chain.GetEntry(h.agentHash)
		a := NewPutAction(AgentEntryType, entry, &Header{})
		resp, err := h.GetValidationResponse(a, h.agentHash)
		So(err, ShouldBeNil)
		So(resp.Type, ShouldEqual, AgentEntryType)
		So(fmt.Sprintf("%v", &resp.Entry), ShouldEqual, fmt.Sprintf("%v", entry))
		So(fmt.Sprintf("%v", resp.Package), ShouldEqual, fmt.Sprintf("%v", Package{}))
	})
}

func TestMakeValidatePackage(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	entry := GobEntry{C: `{"firstName":"Zippy","lastName":"Pinhead"}`}
	h.NewEntry(time.Now(), "evenNumbers", &entry)

	pkg, _ := MakePackage(h, PackagingReq{PkgReqChain: int64(PkgReqChainOptFull)})
	Convey("it should be able to make a validate package", t, func() {
		vpkg, err := MakeValidationPackage(h, &pkg)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", vpkg.Chain), ShouldEqual, fmt.Sprintf("%v", h.chain))
	})

	Convey("it should return an error if the package data was tweaked", t, func() {
		// tweak the agent header
		pkg.Chain = []byte(strings.Replace(string(pkg.Chain), "%agent", "!agent", -1))
		vpkg, err := MakeValidationPackage(h, &pkg)
		So(err, ShouldNotBeNil)
		So(vpkg, ShouldBeNil)

		// restore
		pkg.Chain = []byte(strings.Replace(string(pkg.Chain), "!agent", "%agent", -1))

		// tweak
		pkg.Chain = []byte(strings.Replace(string(pkg.Chain), "Zippy", "Zappy", -1))

		vpkg, err = MakeValidationPackage(h, &pkg)
		So(err, ShouldNotBeNil)

	})
}
