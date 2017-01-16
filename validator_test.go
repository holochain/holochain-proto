package holochain

import (
	"fmt"
	"testing"
	. "github.com/smartystreets/goconvey/convey"

)

func TestNewZygoValidator(t *testing.T) {
	Convey("new should create a validator",t,func(){
		v,err := NewZygoValidator(`(+ 1 1)`)
		z := v.(*ZygoValidator)
		So(err,ShouldBeNil)
 		result,err := z.env.Run()
		So(err,ShouldBeNil)
		So(fmt.Sprintf("%v",result),ShouldEqual,"&{2 <nil>}")
	})
	Convey("new fail to create validator when code is bad",t,func(){
		v,err := NewZygoValidator("(should make a zygo syntax error")
		So(v,ShouldBeNil)
		So(err.Error(),ShouldEqual,"Zygomys error: Error on line 1: parser needs more input\n")
	})
}

func TestZygoValidateEntry(t *testing.T) {
	Convey("should run an entry value against the defined validator",t,func(){
		v,err := NewZygoValidator(`(defn validateEntry [entry] (cond (== entry "fish") true false))`)
		So(err,ShouldBeNil)
		err = v.ValidateEntry(`"cow"`)
		So(err.Error(),ShouldEqual,"Invalid entry")
		err = v.ValidateEntry(`"fish"`)
		So(err,ShouldBeNil)
	})
}
