package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestValidateReceiver(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("VALIDATE_REQUEST should fail if  body isn't a ValidateQuery", t, func() {
		m := h.node.NewMessage(VALIDATE_REQUEST, "fish")
		_, err := ValidateReceiver(h, m)
		So(err.Error(), ShouldEqual, "expected ValidateQuery")
	})
	Convey("VALIDATE_REQUEST should fail if hash doesn't exist", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		m := h.node.NewMessage(VALIDATE_REQUEST, ValidateQuery{H: hash})
		_, err := ValidateReceiver(h, m)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "hash not found")
	})
	Convey("VALIDATE_REQUEST should return entry by hash", t, func() {
		entry := GobEntry{C: "bogus entry data"}
		_, hd, err := h.NewEntry(time.Now(), "evenNumbers", &entry)

		m := h.node.NewMessage(VALIDATE_REQUEST, ValidateQuery{H: hd.EntryLink})
		r, err := ValidateReceiver(h, m)
		So(err, ShouldBeNil)
		So(r.(*ValidateResponse).Type, ShouldEqual, "evenNumbers")
		So(fmt.Sprintf("%v", r.(*ValidateResponse).Entry), ShouldEqual, fmt.Sprintf("%v", &entry))
		So(fmt.Sprintf("%v", r.(*ValidateResponse).Header), ShouldEqual, fmt.Sprintf("%v", *hd))
	})
	Convey("VALIDATELINK_REQUEST should fail if  body isn't a ValidateQuery", t, func() {
		m := h.node.NewMessage(VALIDATELINK_REQUEST, "fish")
		_, err := ValidateReceiver(h, m)
		So(err.Error(), ShouldEqual, "expected ValidateQuery")
	})
	Convey("VALIDATELINK_REQUEST should fail if hash doesn't exist", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		m := h.node.NewMessage(VALIDATELINK_REQUEST, ValidateQuery{H: hash})
		_, err := ValidateReceiver(h, m)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "hash not found")
	})

	entry := GobEntry{C: "bogus entry data"}
	_, hd, _ := h.NewEntry(time.Now(), "evenNumbers", &entry)
	hash := hd.EntryLink

	Convey("VALIDATELINK_REQUEST should return error if hash isn't a linking entry", t, func() {
		m := h.node.NewMessage(VALIDATELINK_REQUEST, ValidateQuery{H: hash})
		_, err := ValidateReceiver(h, m)
		So(err.Error(), ShouldEqual, "hash not of a linking entry")
	})

	Convey("VALIDATELINK_REQUEST should return entry by linking entry hash", t, func() {
		someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
		e := GobEntry{C: someData}
		_, phd, _ := h.NewEntry(time.Now(), "profile", &e)
		profileHash := phd.EntryLink
		e = GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String())}
		_, le, _ := h.NewEntry(time.Now(), "rating", &e)

		m := h.node.NewMessage(VALIDATELINK_REQUEST, ValidateQuery{H: le.EntryLink})
		r, err := ValidateReceiver(h, m)
		So(err, ShouldBeNil)
		vr := r.(*ValidateLinkResponse)
		//	So(vr.Tag, ShouldEqual, "a meta tag")
		//	So(vr.Type, ShouldEqual, "evenNumbers")
		So(fmt.Sprintf("%v", vr), ShouldEqual, "&{rating [{QmdykVTmyPfSaqx4WJQoRHhg7GM7ree8W961pS7uCmhYuJ QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt 4stars}]}")
	})
}
