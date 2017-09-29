package holochain

import (
	//	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestZomeGetEntryDef(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)

	z, _ := h.GetZome("zySampleZome")

	Convey("it should fail on an undefined entry type", t, func() {
		_, err := z.GetEntryDef("bar")
		So(err.Error(), ShouldEqual, "no definition for entry type: bar")
	})
	Convey("it should get a defined entry type", t, func() {
		z := h.nucleus.dna.Zomes[0]
		d, err := z.GetEntryDef("primes")
		So(err, ShouldBeNil)
		So(d.Name, ShouldEqual, "primes")
	})
}

func TestGetFunctionDef(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)

	z, _ := h.GetZome("zySampleZome")

	Convey("it should fail if the fn isn't defined in the DNA", t, func() {
		_, err := z.GetFunctionDef("foo")
		So(err.Error(), ShouldEqual, "unknown exposed function: foo")
	})
	Convey("it should return the Fn structure of a defined fn", t, func() {
		fn, err := z.GetFunctionDef("getDNA")
		So(err, ShouldBeNil)
		So(fn.Name, ShouldEqual, "getDNA")
	})
}
