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
	})
	Convey("VALIDATEMETA_REQUEST should fail if  body isn't a ValidateQuery", t, func() {
		m := h.node.NewMessage(VALIDATEMETA_REQUEST, "fish")
		_, err := ValidateReceiver(h, m)
		So(err.Error(), ShouldEqual, "expected ValidateQuery")
	})
	Convey("VALIDATEMETA_REQUEST should fail if hash doesn't exist", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		m := h.node.NewMessage(VALIDATEMETA_REQUEST, ValidateQuery{H: hash})
		_, err := ValidateReceiver(h, m)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "hash not found")
	})

	entry := GobEntry{C: "bogus entry data"}
	_, hd, _ := h.NewEntry(time.Now(), "evenNumbers", &entry)

	Convey("VALIDATEMETA_REQUEST should return error if hash isn't a meta entry", t, func() {
		m := h.node.NewMessage(VALIDATEMETA_REQUEST, ValidateQuery{H: hd.EntryLink})
		_, err := ValidateReceiver(h, m)
		So(err.Error(), ShouldEqual, "hash not of meta entry")
	})

	Convey("VALIDATEMETA_REQUEST should return entry by meta hash", t, func() {
		me := MetaEntry{M: hd.EntryLink, Tag: "a meta tag"}
		e := GobEntry{C: me}
		_, mehd, err := h.NewEntry(time.Now(), MetaEntryType, &e)
		if err != nil {
			panic(err)
		}
		m := h.node.NewMessage(VALIDATEMETA_REQUEST, ValidateQuery{H: mehd.EntryLink})
		r, err := ValidateReceiver(h, m)
		So(err, ShouldBeNil)
		vr := r.(*ValidateMetaResponse)
		So(vr.Tag, ShouldEqual, "a meta tag")
		So(vr.Type, ShouldEqual, "evenNumbers")
		So(fmt.Sprintf("%v", vr.Entry), ShouldEqual, fmt.Sprintf("%v", &entry))
	})
}
