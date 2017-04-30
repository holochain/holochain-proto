package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
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
		So(err.Error(), ShouldEqual, "JS exec error: (anonymous): Line 1:25 Unexpected token )")
	})

	Convey("it should have an App structure:", t, func() {
		d, _, h := prepareTestChain("test")
		defer cleanupTestDir(d)

		v, err := NewJSNucleus(h, "")
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)

		_, err = z.Run("App.Name")
		So(err, ShouldBeNil)
		s, _ := z.lastResult.ToString()
		So(s, ShouldEqual, h.Name)

		_, err = z.Run("App.DNA.Hash")
		So(err, ShouldBeNil)
		s, _ = z.lastResult.ToString()
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

func TestJSbuildValidate(t *testing.T) {
	e := GobEntry{C: "2"}
	a := NewCommitAction("evenNumbers", &e)
	var header Header
	a.header = &header

	d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatString}

	Convey("it should build commit", t, func() {
		code, err := buildJSValidateAction(a, &d, []string{"fake_src_hash"})
		So(err, ShouldBeNil)
		So(code, ShouldEqual, `validateCommit("evenNumbers","2",{"EntryLink":"","Type":"","Time":"0001-01-01T00:00:00Z"},["fake_src_hash"])`)
	})
}

func TestJSValidateCommit(t *testing.T) {
	a, _ := NewAgent(IPFS, "Joe")
	h := NewHolochain(a, "some/path", "yaml", Zome{})
	h.config.Loggers.App.New(nil)
	hdr := mkTestHeader("evenNumbers")

	Convey("it should be passing in the correct values", t, func() {
		v, err := NewJSNucleus(&h, `function validateCommit(name,entry,header,sources) {debug(name);debug(entry);debug(JSON.stringify(header));debug(JSON.stringify(sources));return true};`)
		So(err, ShouldBeNil)
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatString}
		ShouldLog(&h.config.Loggers.App, `evenNumbers
foo
{"EntryLink":"QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuU2","Time":"1970-01-01T00:00:01Z","Type":"evenNumbers"}
["fakehashvalue"]
`, func() {
			a := NewCommitAction("oddNumbers", &GobEntry{C: "foo"})
			a.header = &hdr
			err = v.ValidateAction(a, &d, []string{"fakehashvalue"})
			So(err, ShouldBeNil)
		})
	})
	Convey("should run an entry value against the defined validator for string data", t, func() {
		v, err := NewJSNucleus(nil, `function validateCommit(name,entry,header,sources) { return (entry=="fish")};`)
		So(err, ShouldBeNil)
		d := EntryDef{Name: "oddNumbers", DataFormat: DataFormatString}

		a := NewCommitAction("oddNumbers", &GobEntry{C: "cow"})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		a = NewCommitAction("oddNumbers", &GobEntry{C: "fish"})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil)
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for js data", t, func() {
		v, err := NewJSNucleus(nil, `function validateCommit(name,entry,header,sources) { return (entry=="fish")};`)
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatRawJS}

		a := NewCommitAction("oddNumbers", &GobEntry{C: "\"cow\""})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		a = NewCommitAction("oddNumbers", &GobEntry{C: "\"fish\""})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil)
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for json data", t, func() {
		v, err := NewJSNucleus(nil, `function validateCommit(name,entry,header,sources) { return (entry.data=="fish")};`)
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatJSON}

		a := NewCommitAction("evenNumbers", &GobEntry{C: `{"data":"cow"}`})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		a = NewCommitAction("evenNumbers", &GobEntry{C: `{"data":"fish"}`})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil)
		So(err, ShouldBeNil)
	})
}

func TestPrepareJSValidateArgs(t *testing.T) {
	d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatString}

	Convey("it should prepare args for commit", t, func() {
		e := GobEntry{C: "2"}
		a := NewCommitAction("evenNumbers", &e)
		var header Header
		a.header = &header
		args, err := prepareJSValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"2",{"EntryLink":"","Type":"","Time":"0001-01-01T00:00:00Z"}`)
	})
	Convey("it should prepare args for put", t, func() {
		e := GobEntry{C: "2"}
		var header Header
		a := NewPutAction("evenNumbers", &e, &header)

		args, err := prepareJSValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"2",{"EntryLink":"","Type":"","Time":"0001-01-01T00:00:00Z"}`)
	})
	Convey("it should prepare args for del", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		a := NewDelAction("evenNumbers", hash)
		args, err := prepareJSValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2"`)
	})

	Convey("it should prepare args for link", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		a := NewLinkAction("evenNumbers", []Link{{Base: "QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5", Link: "QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5", Tag: "fish"}})
		a.validationBase = hash
		args, err := prepareJSValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2",JSON.parse("[{\"Base\":\"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5\",\"Link\":\"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5\",\"Tag\":\"fish\"}]")`)
	})
}

func TestJSSanitize(t *testing.T) {
	Convey("should strip quotes and returns", t, func() {
		So(jsSanitizeString(`"`), ShouldEqual, `\"`)
		So(jsSanitizeString("\"x\ny"), ShouldEqual, "\\\"xy")
	})
}

func TestJSExposeCall(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	zome, _ := h.GetZome("jsSampleZome")
	v, err := h.makeNucleus(zome)
	if err != nil {
		panic(err)
	}
	z := v.(*JSNucleus)
	Convey("should allow calling exposed STRING based functions", t, func() {
		cater, _ := h.GetFunctionDef(zome, "testStrFn1")
		result, err := z.Call(cater, "fish \"zippy\"")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "result: fish \"zippy\"")

		adder, _ := h.GetFunctionDef(zome, "testStrFn2")
		result, err = z.Call(adder, "10")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "12")
	})
	Convey("should allow calling exposed JSON based functions", t, func() {
		times2, _ := h.GetFunctionDef(zome, "testJsonFn1")
		result, err := z.Call(times2, `{"input": 2}`)
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, `{"input":2,"output":4}`)
	})
	Convey("should sanitize against bad strings", t, func() {
		cater, _ := h.GetFunctionDef(zome, "testStrFn1")
		result, err := z.Call(cater, "fish \"\nzippy\"")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "result: fish \"zippy\"")
	})
	Convey("should sanitize against bad JSON", t, func() {
		times2, _ := h.GetFunctionDef(zome, "testJsonFn1")
		result, err := z.Call(times2, "{\"input\n\": 2}")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, `{"input":2,"output":4}`)
	})
	Convey("should allow a function declared with JSON parameter to be called with no parameter", t, func() {
		emptyParametersJson, _ := h.GetFunctionDef(zome, "testJsonFn2")
		result, err := z.Call(emptyParametersJson, "")
		So(err, ShouldBeNil)
		So(result, ShouldEqual, "[{\"a\":\"b\"}]")
	})
}

func TestJSDHT(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
	Convey("get should return hash not found if it doesn't exist", t, func() {
		v, err := NewJSNucleus(h, fmt.Sprintf(`get("%s");`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		So(z.lastResult.String(), ShouldEqual, "HolochainError: hash not found")
	})

	// add an entry onto the chain
	hash = commit(h, "oddNumbers", "7")

	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}

	Convey("get should return entry", t, func() {
		v, err := NewJSNucleus(h, fmt.Sprintf(`get("%s");`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", x.(Entry).Content()), ShouldEqual, `7`)
	})

	profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)

	commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String()))

	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}

	Convey("getlink function should return the Links", t, func() {
		v, err := NewJSNucleus(h, fmt.Sprintf(`getlink("%s","4stars");`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)

		So(z.lastResult.Class(), ShouldEqual, "Object")
		x, err := z.lastResult.Export()
		lqr := x.(*LinkQueryResp)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", lqr.Links[0].H), ShouldEqual, profileHash.String())
	})

	Convey("getlink function with load option should return the Links and entries", t, func() {
		v, err := NewJSNucleus(h, fmt.Sprintf(`getlink("%s","4stars",{Load:true});`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*JSNucleus)
		So(z.lastResult.Class(), ShouldEqual, "Object")
		x, err := z.lastResult.Export()
		lqr := x.(*LinkQueryResp)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", lqr.Links[0].H), ShouldEqual, profileHash.String())
		So(fmt.Sprintf("%v", lqr.Links[0].E), ShouldEqual, `{"firstName":"Zippy","lastName":"Pinhead"}`)
	})
}
