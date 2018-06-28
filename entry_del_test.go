package holochain

import (
  "fmt"
  "testing"
  . "github.com/HC-Interns/holochain-proto/hash"
  . "github.com/smartystreets/goconvey/convey"
)

func TestDelEntrySysValidate(t *testing.T) {
  d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

  Convey("validate del entry should fail if it doesn't match the del entry schema", t, func() {
		err := sysValidateEntry(h, DelEntryDef, &GobEntry{C: ""}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "unexpected end of JSON input")

		err = sysValidateEntry(h, DelEntryDef, &GobEntry{C: `{"Fish":2}`}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Validation Failed: validator %del failed: object property 'Hash' is required")

		err = sysValidateEntry(h, DelEntryDef, &GobEntry{C: `{"Hash": "not-a-hash"}`}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Validation Failed: Error (input isn't valid multihash) when decoding Hash value 'not-a-hash'")

		err = sysValidateEntry(h, DelEntryDef, &GobEntry{C: `{"Hash": 1}`}, nil)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Validation Failed: validator %del failed: object property 'Hash' validation failed: value is not a string (Kind: float64)")
	})

	Convey("validate del entry should succeed on valid entry", t, func() {
		err := sysValidateEntry(h, DelEntryDef, &GobEntry{C: `{"Hash": "QmUfY4WeqD3UUfczjdkoFQGEgCAVNf7rgFfjdeTbr7JF1C","Message": "obsolete"}`}, nil)
		So(err, ShouldBeNil)
	})
}

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
