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
		So(err.Error(), ShouldEqual, "JS exec error: (anonymous): Line 1:41 Unexpected token )")
	})

	Convey("it should have an App structure:", t, func() {
		d, _, h := prepareTestChain("test")
		defer cleanupTestDir(d)

		v, err := NewJSNucleus(h, "")
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)

		_, err = z.Run("App.DNA.Hash")
		So(err, ShouldBeNil)
		s, _ := z.lastResult.ToString()
		So(s, ShouldEqual, h.dnaHash.String())

		_, err = z.Run("App.Agent.Hash")
		So(err, ShouldBeNil)
		s, _ = z.lastResult.ToString()
		So(s, ShouldEqual, h.agentHash.String())

		_, err = z.Run("App.Agent.String")
		So(err, ShouldBeNil)
		s, _ = z.lastResult.ToString()
		So(s, ShouldEqual, h.Agent().Name())

		_, err = z.Run("App.Key.Hash")
		So(err, ShouldBeNil)
		s, _ = z.lastResult.ToString()
		So(s, ShouldEqual, peer.IDB58Encode(h.id))

	})

	Convey("it should have an HC structure:", t, func() {
		d, _, h := prepareTestChain("test")
		defer cleanupTestDir(d)

		v, err := NewJSNucleus(h, "")
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)

		_, err = z.Run("HC.Version")
		So(err, ShouldBeNil)
		s, _ := z.lastResult.ToString()
		So(s, ShouldEqual, VersionStr)

		_, err = z.Run("HC.JSON")
		So(err, ShouldBeNil)
		i, _ := z.lastResult.ToInteger()
		So(i, ShouldEqual, JSON)

		_, err = z.Run("HC.STRING")
		So(err, ShouldBeNil)
		i, _ = z.lastResult.ToInteger()
		So(i, ShouldEqual, STRING)
	})

	Convey("should have the built in functions:", t, func() {
		d, _, h := prepareTestChain("test")
		defer cleanupTestDir(d)

		v, err := NewJSNucleus(h, "")
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)

		Convey("property", func() {
			_, err = z.Run(`property("description")`)
			So(err, ShouldBeNil)
			s, _ := z.lastResult.ToString()
			So(s, ShouldEqual, "a bogus test holochain")

			ShouldLog(&infoLog, "Warning: Getting special properties via property() is deprecated as of 3. Returning nil values.  Use App* instead\n", func() {
				_, err = z.Run(`property("` + ID_PROPERTY + `")`)
				So(err, ShouldBeNil)
			})

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
		d := EntryDef{Name: "myData", DataFormat: DataFormatString}
		err = v.ValidateEntry(&d, &GobEntry{C: "cow"}, &p)
		So(err.Error(), ShouldEqual, "Invalid entry: cow")
		err = v.ValidateEntry(&d, &GobEntry{C: "fish"}, &p)
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for js data", t, func() {
		v, err := NewJSNucleus(nil, `function validate(name,entry,meta) { return (entry=="fish")};`)
		d := EntryDef{Name: "myData", DataFormat: DataFormatRawJS}
		err = v.ValidateEntry(&d, &GobEntry{C: "\"cow\""}, &p)
		So(err.Error(), ShouldEqual, "Invalid entry: \"cow\"")
		err = v.ValidateEntry(&d, &GobEntry{C: "\"fish\""}, &p)
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for json data", t, func() {
		v, err := NewJSNucleus(nil, `function validate(name,entry,meta) { return (entry.data=="fish")};`)
		d := EntryDef{Name: "myData", DataFormat: DataFormatJSON}
		err = v.ValidateEntry(&d, &GobEntry{C: `{"data":"cow"}`}, &p)
		So(err.Error(), ShouldEqual, `Invalid entry: {"data":"cow"}`)
		err = v.ValidateEntry(&d, &GobEntry{C: `{"data":"fish"}`}, &p)
		So(err, ShouldBeNil)
	})
}

func TestJSSanitize(t *testing.T) {
	Convey("should strip quotes and returns", t, func() {
		So(jsSanitizeString(`"`), ShouldEqual, `\"`)
		So(jsSanitizeString("\"x\ny"), ShouldEqual, "\\\"xy")
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
expose("emptyParametersJson",HC.JSON);
function emptyParametersJson(x){ return [{a:'b'}] };
`)
		So(err, ShouldBeNil)
		z = v.(*JSNucleus)
	})

	Convey("should build up interfaces list", t, func() {
		i := z.Interfaces()
		So(fmt.Sprintf("%v", i), ShouldEqual, "[{cater 0} {adder 0} {jtest 1} {emptyParametersJson 1}]")
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
	Convey("should sanitize against bad strings", t, func() {
		result, err := z.Call("cater", "fish \"\nzippy\"")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "result: fish \"zippy\"")
	})
	Convey("should sanitize against bad JSON", t, func() {
		result, err := z.Call("jtest", "{\"input\n\": 2}")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, `{"input":2,"output":4}`)
	})
	Convey("should allow a function declared with JSON parameter to be called with no parameter", t, func() {
		result, err := z.Call("emptyParametersJson", "")
		So(err, ShouldBeNil)
		So(result, ShouldEqual, "[{\"a\":\"b\"}]")
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

		if err := h.dht.simHandlePutReqs(); err != nil {
			panic(err)
		}

		v, err = NewJSNucleus(h, fmt.Sprintf(`get ("%s");`, hash.String()))
		So(err, ShouldBeNil)
		z = v.(*JSNucleus)
		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", x.(Entry).Content()), ShouldEqual, `7`)
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

	if err := h.dht.simHandlePutReqs(); err != nil {
		panic(err)
	}

	Convey("it should have a getmeta function", t, func() {
		v, err := NewJSNucleus(h, fmt.Sprintf(`getmeta("%s","myMetaTag");`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		So(z.lastResult.Class(), ShouldEqual, "Object")
		x, err := z.lastResult.Export()
		mqr := x.(MetaQueryResp)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", mqr.Entries[0].E.Content()), ShouldEqual, `{"firstName":"Zippy","lastName":"Pinhead"}`)
	})

}
