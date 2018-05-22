package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCreateRibosome(t *testing.T) {
	Convey("should fail to create a ribosome based from bad ribosome type", t, func() {
		_, err := CreateRibosome(nil, &Zome{RibosomeType: "foo", Code: "some code"})
		So(err.Error(), ShouldEqual, "Invalid ribosome name. Must be one of: js, zygo")
	})
	Convey("should create a ribosome based from a good schema type", t, func() {
		v, err := CreateRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(+ 1 1)`})
		z := v.(*ZygoRibosome)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", z.lastResult), ShouldEqual, "&{2 <nil>}")
	})
}

func TestValidExposure(t *testing.T) {
	Convey("public context for zome only functions should be invalid", t, func() {
		fn := FunctionDef{} // zome only is default
		So(fn.ValidExposure(PUBLIC_EXPOSURE), ShouldBeFalse)
		So(fn.ValidExposure(ZOME_EXPOSURE), ShouldBeTrue)
	})
	Convey("public context for public functions should be valid", t, func() {
		fn := FunctionDef{Exposure: PUBLIC_EXPOSURE}
		So(fn.ValidExposure(PUBLIC_EXPOSURE), ShouldBeTrue)
		So(fn.ValidExposure(ZOME_EXPOSURE), ShouldBeTrue)
	})
}

func TestValidationFailedErr(t *testing.T) {
	Convey("it should build the default validation failed err", t, func() {
		So(ValidationFailed(), ShouldEqual, ValidationFailedErr)
	})
	Convey("it should build the custom validation failed err", t, func() {
		So(ValidationFailed("just because").Error(), ShouldEqual, ValidationFailedErrMsg+": just because")
	})
	Convey("it should test errors", t, func() {
		So(IsValidationFailedErr(ValidationFailed("just because")), ShouldBeTrue)
		So(IsValidationFailedErr(ValidationFailed()), ShouldBeTrue)
		So(IsValidationFailedErr(ErrHashNotFound), ShouldBeFalse)
	})

}
