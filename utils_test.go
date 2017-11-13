package holochain

import (
	"bytes"
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestUtilsEncodeDecode(t *testing.T) {
	var data, data1 struct {
		A int
		B string
	}
	data.A = 314
	data.B = "fish"
	Convey("json", t, func() {
		var b bytes.Buffer
		err := Encode(&b, "json", data)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", string(b.Bytes())), ShouldEqual, "{\n    \"A\": 314,\n    \"B\": \"fish\"\n}\n")
		err = Decode(&b, "json", &data1)
		So(err, ShouldBeNil)
		So(data.A, ShouldEqual, data1.A)
		So(data.B, ShouldEqual, data1.B)
	})
	data1.A = 0
	data1.B = ""
	Convey("yaml", t, func() {
		var b bytes.Buffer
		err := Encode(&b, "yaml", data)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", string(b.Bytes())), ShouldEqual, "A: 314\nB: fish\n")
		err = Decode(&b, "yaml", &data1)
		So(err, ShouldBeNil)
		So(data.A, ShouldEqual, data1.A)
		So(data.B, ShouldEqual, data1.B)
	})
	data1.A = 0
	data1.B = ""
	Convey("toml", t, func() {
		var b bytes.Buffer
		err := Encode(&b, "toml", data)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", string(b.Bytes())), ShouldEqual, "A = 314\nB = \"fish\"\n")
		err = Decode(&b, "toml", &data1)
		So(err, ShouldBeNil)
		So(data.A, ShouldEqual, data1.A)
		So(data.B, ShouldEqual, data1.B)
	})
}

func TestTicker(t *testing.T) {
	counter := 0
	stopper := Ticker(10*time.Millisecond, func() {
		counter += 1
	})
	time.Sleep(25 * time.Millisecond)
	stopper <- true
	Convey("it should have ticked twice", t, func() {
		So(counter, ShouldEqual, 2)
	})
}

func TestEncodingFormat(t *testing.T) {
	Convey("it should return valid formats", t, func() {
		So(EncodingFormat("dog.json"), ShouldEqual, "json")
		So(EncodingFormat("/fish/cow/dog.yaml"), ShouldEqual, "yaml")
		So(EncodingFormat("fish.toml"), ShouldEqual, "toml")
		So(EncodingFormat("fish.xml"), ShouldEqual, "")
	})
}
