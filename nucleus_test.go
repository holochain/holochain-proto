package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCreateNucleus(t *testing.T) {
	Convey("should fail to create a nucleus based from bad nucleus type", t, func() {
		_, err := CreateNucleus(nil, "non-existent-nucleus-type", "some code")
		So(err.Error(), ShouldEqual, "Invalid nucleus name. Must be one of: js, zygo")
	})
	Convey("should create a nucleus based from a good schema type", t, func() {
		v, err := CreateNucleus(nil, ZygoNucleusType, `(+ 1 1)`)
		z := v.(*ZygoNucleus)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", z.lastResult), ShouldEqual, "&{2 <nil>}")
	})
}
