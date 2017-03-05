package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/robertkrimen/otto"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestNewJSNucleus(t *testing.T) {
	Convey("new should create a nucleus", t, func() {
		v, err := NewJSNucleus(nil, `1 + 1`)
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		i, _ := z.lastResult.ToInteger()
		So(i, ShouldEqual, 2)
	})
	Convey("new fail to create nucleus when code is bad", t, func() {
		v, err := NewJSNucleus(nil, "1+ )")
		So(v, ShouldBeNil)
		So(err.Error(), ShouldEqual, "JS exec error: (anonymous): Line 1:45 Unexpected token )")
	})
	Convey("should have the built in functions:", t, func() {
		d, _, h := prepareTestChain("test")
		defer cleanupTestDir(d)

		v, err := NewJSNucleus(h, "")
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		Convey("version", func() {
			_, err = z.Run("version")
			So(err, ShouldBeNil)
			s, _ := z.lastResult.ToString()
			So(s, ShouldEqual, "0.0.1")
		})
		Convey("property", func() {
			_, err = z.Run(`property("description")`)
			So(err, ShouldBeNil)
			s, _ := z.lastResult.ToString()
			So(s, ShouldEqual, "a bogus test holochain")

			_, err = z.Run(`property("` + ID_PROPERTY + `")`)
			So(err, ShouldBeNil)
			id, _ := h.ID()
			So(z.lastResult.String(), ShouldEqual, id.String())

			_, err = z.Run(`property("` + AGENT_ID_PROPERTY + `")`)
			So(err, ShouldBeNil)
			aid := peer.IDB58Encode(h.node.HashAddr)
			So(z.lastResult.String(), ShouldEqual, aid)

			_, err = z.Run(`property ("` + AGENT_NAME_PROPERTY + `")`)
			So(err, ShouldBeNil)
			aid = string(h.Agent().ID())
			So(z.lastResult.String(), ShouldEqual, aid)

		})
	})
}

func TestJSGenesis(t *testing.T) {
	Convey("it should fail if the init function returns false", t, func() {
		z, _ := NewJSNucleus(nil, `function genesis() {return false}`)
		err := z.ChainGenesis()
		So(err.Error(), ShouldEqual, "genesis failed")
	})
	Convey("it should work if the genesis function returns true", t, func() {
		z, _ := NewJSNucleus(nil, `function genesis() {return true}`)
		err := z.ChainGenesis()
		So(err, ShouldBeNil)
	})
}

func TestJSValidateEntry(t *testing.T) {
	p := ValidationProps{}
	Convey("should run an entry value against the defined validator for string data", t, func() {
		v, err := NewJSNucleus(nil, `function validate(name,entry,meta) { return (entry=="fish")};`)
		So(err, ShouldBeNil)
		d := EntryDef{Name: "myData", DataFormat: "string"}
		err = v.ValidateEntry(&d, &GobEntry{C: "cow"}, &p)
		So(err.Error(), ShouldEqual, "Invalid entry: cow")
		err = v.ValidateEntry(&d, &GobEntry{C: "fish"}, &p)
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for js data", t, func() {
		v, err := NewJSNucleus(nil, `function validate(name,entry,meta) { return (entry=="fish")};`)
		d := EntryDef{Name: "myData", DataFormat: "js"}
		err = v.ValidateEntry(&d, &GobEntry{C: "\"cow\""}, &p)
		So(err.Error(), ShouldEqual, "Invalid entry: \"cow\"")
		err = v.ValidateEntry(&d, &GobEntry{C: "\"fish\""}, &p)
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for json data", t, func() {
		v, err := NewJSNucleus(nil, `function validate(name,entry,meta) { return (entry.data=="fish")};`)
		d := EntryDef{Name: "myData", DataFormat: "JSON"}
		err = v.ValidateEntry(&d, &GobEntry{C: `{"data":"cow"}`}, &p)
		So(err.Error(), ShouldEqual, `Invalid entry: {"data":"cow"}`)
		err = v.ValidateEntry(&d, &GobEntry{C: `{"data":"fish"}`}, &p)
		So(err, ShouldBeNil)
	})
}

func TestJSExposeCall(t *testing.T) {
	var z *JSNucleus
	Convey("should run", t, func() {
		v, err := NewJSNucleus(nil, `
expose("cater",HC.STRING);
function cater(x) {return "result: "+x};
expose("adder",HC.STRING);
function adder(x){ return parseInt(x)+2};
expose("jtest",HC.JSON);
function jtest(x){ x.output = x.input*2; return x;};
`)
		So(err, ShouldBeNil)
		z = v.(*JSNucleus)
	})

	Convey("should build up interfaces list", t, func() {
		i := z.Interfaces()
		So(fmt.Sprintf("%v", i), ShouldEqual, "[{cater 0} {adder 0} {jtest 1}]")
	})
	Convey("should allow calling exposed STRING based functions", t, func() {
		result, err := z.Call("cater", "fish \"zippy\"")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "result: fish \"zippy\"")

		result, err = z.Call("adder", "10")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "12")
	})
	Convey("should allow calling exposed JSON based functions", t, func() {
		result, err := z.Call("jtest", `{"input": 2}`)
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, `{"input":2,"output":4}`)
	})
}

func TestJSDHT(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	data := "7"

	// add an entry onto the chain
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: data}
	_, hd, err := h.NewEntry(now, "myOdds", &e)
	if err != nil {
		panic(err)
	}

	hash := hd.EntryLink
	Convey("it should have a put function", t, func() {
		v, err := NewJSNucleus(h, fmt.Sprintf(`put("%s");`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		So(err, ShouldBeNil)
		So(z.lastResult.String(), ShouldEqual, otto.UndefinedValue().String())
	})

	Convey("it should have a get function", t, func() {
		v, err := NewJSNucleus(h, fmt.Sprintf(`get ("%s");`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		So(z.lastResult.String(), ShouldEqual, "HolochainError: hash not found")

		if err := h.dht.handlePutReqs(); err != nil {
			panic(err)
		}

		v, err = NewJSNucleus(h, fmt.Sprintf(`get ("%s");`, hash.String()))
		So(err, ShouldBeNil)
		z = v.(*JSNucleus)
		So(z.lastResult.String(), ShouldEqual, `"7"`)
	})

	e = GobEntry{C: `{"firstName":"Zippy","lastName":"Pinhead"}`}
	_, mhd, _ := h.NewEntry(now, "profile", &e)
	metaHash := mhd.EntryLink
	//b, _ := e.Marshal()

	Convey("it should have a putmeta function", t, func() {
		v, err := NewJSNucleus(h, fmt.Sprintf(`putmeta("%s","%s","myMetaTag");`, hash.String(), metaHash.String()))
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		So(err, ShouldBeNil)
		So(z.lastResult.String(), ShouldEqual, otto.UndefinedValue().String())
	})

	if err := h.dht.handlePutReqs(); err != nil {
		panic(err)
	}

	Convey("it should have a getmeta function", t, func() {
		v, err := NewJSNucleus(h, fmt.Sprintf(`getmeta("%s","myMetaTag");`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		So(z.lastResult.String(), ShouldEqual, `[{"C":"{\"firstName\":\"Zippy\",\"lastName\":\"Pinhead\"}"}]`)
	})

}
