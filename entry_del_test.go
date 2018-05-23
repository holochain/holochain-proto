package holochain

import (
  "fmt"
  "testing"
  . "github.com/holochain/holochain-proto/hash"
  . "github.com/smartystreets/goconvey/convey"
)

func TestDelEntryToJSON(t *testing.T) {
	hashStr := "QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx"
	hash, _ := NewHash(hashStr)
	e1 := DelEntry{
		Hash:    hash,
		Message: "my message",
	}

	var j string
	var err error
	Convey("it should convert to JSON", t, func() {
		j, err = e1.ToJSON()
		So(err, ShouldBeNil)
		So(j, ShouldEqual, fmt.Sprintf(`{"Hash":"%s","Message":"my message"}`, hashStr))
	})

	Convey("it should convert from JSON", t, func() {
		e2, err := DelEntryFromJSON(j)
		So(err, ShouldBeNil)
		So(e2.Message, ShouldEqual, e1.Message)
		So(e2.Hash.String(), ShouldEqual, e1.Hash.String())
	})

}
