package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCreateNucleus(t *testing.T) {
	Convey("should fail to create a nucleus based from bad schema type", t, func() {
		_, err := CreateNucleus("non-existent-schema", "some code")
		So(err.Error(), ShouldEqual, "Invalid nucleus name. Must be one of: zygo")
	})
	Convey("should create a nucleus based from a good schema type", t, func() {
		v, err := CreateNucleus(ZygoSchemaType, `(+ 1 1)`)
		z := v.(*ZygoNucleus)
		So(err, ShouldBeNil)
		result, err := z.env.Run()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", result), ShouldEqual, "&{2 <nil>}")
	})
}
