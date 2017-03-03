package holochain

import (
	"fmt"
	zygo "github.com/glycerine/zygomys/repl"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestNewZygoNucleus(t *testing.T) {
	Convey("new should create a nucleus", t, func() {
		v, err := NewZygoNucleus(nil, `(+ 1 1)`)
		z := v.(*ZygoNucleus)
		So(err, ShouldBeNil)
		So(z.lastResult.(*zygo.SexpInt).Val, ShouldEqual, 2)
	})
	Convey("new fail to create nucleus when code is bad", t, func() {
		v, err := NewZygoNucleus(nil, "(should make a zygo syntax error")
		So(v, ShouldBeNil)
		So(err.Error(), ShouldEqual, "Zygomys load error: Error on line 1: parser needs more input\n")
	})
	Convey("should have the built in functions:", t, func() {
		d, _, h := setupTestChain("test")
		defer cleanupTestDir(d)

		v, err := NewZygoNucleus(h, "")
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
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
		Convey("isprime", func() {
			_, err = z.Run(`(isprime 100)`)
			So(err, ShouldBeNil)
			So(z.lastResult.(*zygo.SexpBool).Val, ShouldEqual, false)
			_, err = z.Run(`(isprime 7)`)
			So(err, ShouldBeNil)
			So(z.lastResult.(*zygo.SexpBool).Val, ShouldEqual, true)
			_, err = z.Run(`(isprime "fish")`)
			So(err.Error(), ShouldEqual, "Zygomys exec error: Error calling 'isprime': argument to isprime should be int")
		})
		Convey("property", func() {
			_, err = z.Run(`(property "description")`)
			So(err, ShouldBeNil)
			So(z.lastResult.(*zygo.SexpStr).S, ShouldEqual, "a bogus test holochain")
		})
	})
}

func TestZygoValidateEntry(t *testing.T) {
	Convey("should run an entry value against the defined validator for string data", t, func() {
		v, err := NewZygoNucleus(nil, `(defn validate [name entry] (cond (== entry "fish") true false))`)
		d := EntryDef{Name: "myData", DataFormat: "string"}
		err = v.ValidateEntry(&d, "cow")
		So(err.Error(), ShouldEqual, "Invalid entry: cow")
		err = v.ValidateEntry(&d, "fish")
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for zygo data", t, func() {
		v, err := NewZygoNucleus(nil, `(defn validate [name entry] (cond (== entry "fish") true false))`)
		d := EntryDef{Name: "myData", DataFormat: "zygo"}
		err = v.ValidateEntry(&d, "\"cow\"")
		So(err.Error(), ShouldEqual, "Invalid entry: \"cow\"")
		err = v.ValidateEntry(&d, "\"fish\"")
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for json data", t, func() {
		v, err := NewZygoNucleus(nil, `(defn validate [name entry] (cond (== (hget entry data:) "fish") true false))`)
		d := EntryDef{Name: "myData", DataFormat: "JSON"}
		err = v.ValidateEntry(&d, `{"data":"cow"}`)
		So(err.Error(), ShouldEqual, `Invalid entry: {"data":"cow"}`)
		err = v.ValidateEntry(&d, `{"data":"fish"}`)
		So(err, ShouldBeNil)
	})
}

func TestZygoExposeCall(t *testing.T) {
	var z *ZygoNucleus
	Convey("should run", t, func() {
		v, err := NewZygoNucleus(nil, `
(expose "cater" STRING)
(defn cater [x] (concat "result: " x))
(expose "adder" STRING)
(defn adder [x] (+ (atoi x) 2))
(expose "jtest" JSON)
(defn jtest [x] (begin (hset x output: (* (-> x input:) 2)) x))
`)

		So(err, ShouldBeNil)
		z = v.(*ZygoNucleus)
		_, err = z.env.Run()
		So(err, ShouldBeNil)
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
		So(string(result.([]byte)), ShouldEqual, `{"Atype":"hash", "input":2, "output":4, "zKeyOrder":["input", "output"]}`)
	})
}

func TestZygoDHT(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	data := "7"

	// add an entry onto the chain
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: data}
	_, hd, err := h.NewEntry(now, "myData", &e)
	if err != nil {
		panic(err)
	}

	hash := hd.EntryLink
	Convey("it should have a put function", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(put "%s")`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, "ok")
	})

	Convey("it should have a get function", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(get "%s")`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("error"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, "hash not found")

		if err := h.dht.handlePutReqs(); err != nil {
			panic(err)
		}
		v, err = NewZygoNucleus(h, fmt.Sprintf(`(get "%s")`, hash.String()))
		So(err, ShouldBeNil)
		z = v.(*ZygoNucleus)
		r, err = z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `"7"`)

	})

	e = GobEntry{C: "some meta data"}
	_, mhd, _ := h.NewEntry(now, "myMetaData", &e)
	metaHash := mhd.EntryLink
	//b, _ := e.Marshal()

	Convey("it should have a putmeta function", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(putmeta "%s" "%s" "myMetaType")`, hash.String(), metaHash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, "ok")
	})

	if err := h.dht.handlePutReqs(); err != nil {
		panic(err)
	}

	Convey("it should have a getmeta function", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(getmeta "%s" "myMetaType")`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `[{"H":{"H":"EiD8xq07JbMzjffizM8B1dhXNca2zxIDel3yIxD03EQp+w=="},"T":"QmfMPAEdN1BB9imcz97NsaYYaWEN3baC5aSDXqJSiWt4e6","V":"DhAABnN0cmluZwwDAAE3"}]`)
	})
}
