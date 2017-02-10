package holochain

import (
	"fmt"
	zygo "github.com/glycerine/zygomys/repl"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestNewZygoNucleus(t *testing.T) {
	Convey("new should create a nucleus", t, func() {
		v, err := NewZygoNucleus(nil, `(+ 1 1)`)
		z := v.(*ZygoNucleus)
		So(err, ShouldBeNil)
		So(err, ShouldBeNil)
		So(z.lastResult.(*zygo.SexpInt).Val, ShouldEqual, 2)
	})
	Convey("new fail to create nucleus when code is bad", t, func() {
		v, err := NewZygoNucleus(nil, "(should make a zygo syntax error")
		So(v, ShouldBeNil)
		So(err.Error(), ShouldEqual, "Zygomys load error: Error on line 1: parser needs more input\n")
	})
	Convey("should have the built in functions:", t, func() {
		v, err := NewZygoNucleus(nil, "")
		z := v.(*ZygoNucleus)
		So(err, ShouldBeNil)
		Convey("version", func() {
			_, err = z.Run("(version)")
			So(err, ShouldBeNil)
			So(z.lastResult.(*zygo.SexpStr).S, ShouldEqual, "0.0.1")
		})
		Convey("atoi", func() {
			_, err = z.Run(`(atoi "3141")`)
			So(err, ShouldBeNil)
			So(z.lastResult.(*zygo.SexpInt).Val, ShouldEqual, 3141)
			_, err = z.Run(`(atoi 1)`)
			So(err.Error(), ShouldEqual, "Zygomys exec error: Error calling 'atoi': argument to atoi should be string")
		})
	})
}

func TestZygoValidateEntry(t *testing.T) {
	v, err := NewZygoNucleus(nil, `(defn validate [name entry] (cond (== entry "fish") true false))`)
	Convey("should run an entry value against the defined validator for string data", t, func() {
		d := EntryDef{Name: "myData", Schema: "string"}
		err = v.ValidateEntry(&d, "cow")
		So(err.Error(), ShouldEqual, "Invalid entry:\"cow\"")
		err = v.ValidateEntry(&d, "fish")
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for zygo data", t, func() {
		d := EntryDef{Name: "myData", Schema: "zygo"}
		err = v.ValidateEntry(&d, "\"cow\"")
		So(err.Error(), ShouldEqual, "Invalid entry:\"cow\"")
		err = v.ValidateEntry(&d, "\"fish\"")
		So(err, ShouldBeNil)
	})
}

func TestZygoExposeCall(t *testing.T) {
	var z *ZygoNucleus
	Convey("should run", t, func() {
		v, err := NewZygoNucleus(nil, `
(expose "cater" STRING)
(defn cater [x] (concat "result: " x))
`)

		So(err, ShouldBeNil)
		z = v.(*ZygoNucleus)
		_, err = z.env.Run()
		So(err, ShouldBeNil)
	})

	Convey("should build up interfaces list", t, func() {
		i := z.Interfaces()
		So(fmt.Sprintf("%v", i), ShouldEqual, "[{cater 0}]")
	})
	Convey("should allow calling exposed functions", t, func() {
		result, err := z.Call("cater", "fish")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "result: fish")
	})
}
