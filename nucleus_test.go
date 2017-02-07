package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestNewZygoNucleus(t *testing.T) {
	Convey("new should create a validator", t, func() {
		v, err := NewZygoNucleus(`(+ 1 1)`)
		z := v.(*ZygoNucleus)
		So(err, ShouldBeNil)
		result, err := z.env.Run()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", result), ShouldEqual, "&{2 <nil>}")
	})
	Convey("new fail to create validator when code is bad", t, func() {
		v, err := NewZygoNucleus("(should make a zygo syntax error")
		So(v, ShouldBeNil)
		So(err.Error(), ShouldEqual, "Zygomys error: Error on line 1: parser needs more input\n")
	})
}

func TestCreateNucleus(t *testing.T) {
	Convey("should fail to create a validator based from bad schema type", t, func() {
		_, err := CreateNucleus("non-existent-schema", "some code")
		So(err.Error(), ShouldEqual, "Invalid validator name. Must be one of: zygo")
	})
	Convey("should create a validator based from a good schema type", t, func() {
		v, err := CreateNucleus(ZygoSchemaType, `(+ 1 1)`)
		z := v.(*ZygoNucleus)
		So(err, ShouldBeNil)
		result, err := z.env.Run()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", result), ShouldEqual, "&{2 <nil>}")
	})
}

func TestZygoValidateEntry(t *testing.T) {
	Convey("should run an entry value against the defined validator", t, func() {
		v, err := NewZygoNucleus(`(defn validateEntry [entry] (cond (== entry "fish") true false))`)
		So(err, ShouldBeNil)
		err = v.ValidateEntry(`"cow"`)
		So(err.Error(), ShouldEqual, "Invalid entry:\"cow\"")
		err = v.ValidateEntry(`"fish"`)
		So(err, ShouldBeNil)
	})
}
