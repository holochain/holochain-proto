package holochain

import (
	"bytes"
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"strings"
	"testing"
	"time"
)

func TestValidateReceiver(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

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

func TestValidateMakePackage(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	var emptyStringList []string

	Convey("it should be able to make a full chain package", t, func() {
		req := PackagingReq{PkgReqChain: int64(PkgReqChainOptFull)}
		pkg, err := MakePackage(h, req)
		So(err, ShouldBeNil)

		var b bytes.Buffer
		h.chain.MarshalChain(&b, ChainMarshalFlagsOmitDNA, emptyStringList, emptyStringList)
		So(string(pkg.Chain), ShouldEqual, string(b.Bytes()))
	})
	Convey("it should be able to make a headers only chain package", t, func() {
		req := PackagingReq{PkgReqChain: int64(PkgReqChainOptHeaders)}
		pkg, err := MakePackage(h, req)
		So(err, ShouldBeNil)

		var b bytes.Buffer
		h.chain.MarshalChain(&b, ChainMarshalFlagsNoEntries+ChainMarshalFlagsOmitDNA, emptyStringList, emptyStringList)
		So(string(pkg.Chain), ShouldEqual, string(b.Bytes()))
	})
	Convey("it should be able to make an entries only chain package", t, func() {
		req := PackagingReq{PkgReqChain: int64(PkgReqChainOptEntries)}
		pkg, err := MakePackage(h, req)
		So(err, ShouldBeNil)

		var b bytes.Buffer
		h.chain.MarshalChain(&b, ChainMarshalFlagsNoHeaders+ChainMarshalFlagsOmitDNA, emptyStringList, emptyStringList)
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
		h.chain.MarshalChain(&b, ChainMarshalFlagsOmitDNA, []string{"oddNumbers"}, emptyStringList)
		So(string(pkg.Chain), ShouldEqual, string(b.Bytes()))
	})

	Convey("it should not contain the real contents of private entries", t, func() {
		entry := GobEntry{C: "secret message"}
		h.NewEntry(time.Now(), "privateData", &entry)

		req := PackagingReq{PkgReqChain: int64(PkgReqChainOptFull)}
		pkg, err := MakePackage(h, req)
		So(err, ShouldBeNil)

		_, c1, err := UnmarshalChain(h.hashSpec, bytes.NewBuffer(pkg.Chain))
		So(err, ShouldBeNil)
		So(c1.Entries[2].Content(), ShouldEqual, "2") //from previous test cases
		So(c1.Entries[3].Content(), ShouldEqual, "3") //from previous test cases
		So(c1.Entries[4].Content(), ShouldNotEqual, "secret message")
		So(c1.Entries[4].Content(), ShouldEqual, ChainMarshalPrivateEntryRedacted)
	})

}

func TestGetValidationResponse(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	var emptyStringList []string

	hash := commit(h, "oddNumbers", "3")

	Convey("entry types should return packages based on definition", t, func() {
		entry, _, err := h.chain.GetEntry(hash)
		if err != nil {
			panic(err)
		}
		a := NewPutAction("oddNumbers", entry, &Header{})
		resp, err := h.GetValidationResponse(a, hash)
		So(err, ShouldBeNil)
		So(resp.Type, ShouldEqual, "oddNumbers")
		So(fmt.Sprintf("%v", &resp.Entry), ShouldEqual, fmt.Sprintf("%v", entry))
		var b bytes.Buffer
		h.chain.MarshalChain(&b, ChainMarshalFlagsOmitDNA, emptyStringList, emptyStringList)
		So(fmt.Sprintf("%v", string(resp.Package.Chain)), ShouldEqual, fmt.Sprintf("%v", string(b.Bytes())))
	})

	Convey("it should fail on the DNA (can't validate DNA as it's what determines what's valid)", t, func() {
		entry, _, err := h.chain.GetEntry(h.dnaHash)
		a := NewPutAction(AgentEntryType, entry, &Header{})
		_, err = h.GetValidationResponse(a, h.dnaHash)
		So(err, ShouldEqual, ErrNotValidForDNAType)

	})

	Convey("agent entry type should return the type chain in the package", t, func() {
		entry, _, err := h.chain.GetEntry(h.agentHash)
		a := NewPutAction(AgentEntryType, entry, &Header{})
		resp, err := h.GetValidationResponse(a, h.agentHash)
		So(err, ShouldBeNil)
		So(resp.Type, ShouldEqual, AgentEntryType)
		So(fmt.Sprintf("%v", &resp.Entry), ShouldEqual, fmt.Sprintf("%v", entry))

		types := []string{AgentEntryType}
		var b bytes.Buffer
		h.chain.MarshalChain(&b, ChainMarshalFlagsOmitDNA, types, emptyStringList)
		So(fmt.Sprintf("%v", string(resp.Package.Chain)), ShouldEqual, fmt.Sprintf("%v", string(b.Bytes())))
	})

	Convey("key entry type should return empty package with pubkey as entry", t, func() {
		hash := HashFromPeerID(h.nodeID)
		a := NewPutAction(KeyEntryType, nil, &Header{})
		resp, err := h.GetValidationResponse(a, hash)
		So(err, ShouldBeNil)
		So(resp.Type, ShouldEqual, KeyEntryType)

		pk, err := ic.MarshalPublicKey(h.agent.PubKey())
		if err != nil {
			panic(err)
		}

		So(fmt.Sprintf("%v", resp.Entry.Content()), ShouldEqual, fmt.Sprintf("%v", pk))
		So(fmt.Sprintf("%v", resp.Package), ShouldEqual, fmt.Sprintf("%v", Package{}))
	})
}

func TestMakeValidatePackage(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	entry := GobEntry{C: `{"firstName":"Zippy","lastName":"Pinhead"}`}
	h.NewEntry(time.Now().Round(0), "evenNumbers", &entry)

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
