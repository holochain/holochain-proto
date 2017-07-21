package holochain

import (
	"encoding/json"
	"fmt"
	"github.com/robertkrimen/otto"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestNewJSRibosome(t *testing.T) {
	Convey("new should create a ribosome", t, func() {
		v, err := NewJSRibosome(nil, &Zome{RibosomeType: JSRibosomeType, Code: `1 + 1`})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		i, _ := z.lastResult.ToInteger()
		So(i, ShouldEqual, 2)
	})
	Convey("new fail to create ribosome when code is bad", t, func() {
		v, err := NewJSRibosome(nil, &Zome{RibosomeType: JSRibosomeType, Code: "\n1+ )"})
		So(v, ShouldBeNil)
		So(err.Error(), ShouldEqual, "JS exec error: (anonymous): Line 2:4 Unexpected token )")
	})

	Convey("it should have an App structure:", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestDir(d)

		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)

		_, err = z.Run("App.Name")
		So(err, ShouldBeNil)
		s, _ := z.lastResult.ToString()
		So(s, ShouldEqual, h.nucleus.dna.Name)

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
		So(s, ShouldEqual, h.Agent().Identity())

		_, err = z.Run("App.Key.Hash")
		So(err, ShouldBeNil)
		s, _ = z.lastResult.ToString()
		So(s, ShouldEqual, h.nodeIDStr)

	})

	Convey("it should have an HC structure:", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestDir(d)

		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)

		_, err = z.Run("HC.Version")
		So(err, ShouldBeNil)
		s, _ := z.lastResult.ToString()
		So(s, ShouldEqual, VersionStr)

		_, err = z.Run("HC.Status.Deleted")
		So(err, ShouldBeNil)
		i, _ := z.lastResult.ToInteger()
		So(i, ShouldEqual, StatusDeleted)

		_, err = z.Run("HC.Status.Live")
		So(err, ShouldBeNil)
		i, _ = z.lastResult.ToInteger()
		So(i, ShouldEqual, StatusLive)

		_, err = z.Run("HC.Status.Rejected")
		So(err, ShouldBeNil)
		i, _ = z.lastResult.ToInteger()
		So(i, ShouldEqual, StatusRejected)

		_, err = z.Run("HC.Status.Modified")
		So(err, ShouldBeNil)
		i, _ = z.lastResult.ToInteger()
		So(i, ShouldEqual, StatusModified)

		_, err = z.Run("HC.Status.Any")
		So(err, ShouldBeNil)
		i, _ = z.lastResult.ToInteger()
		So(i, ShouldEqual, StatusAny)
	})

	Convey("should have the built in functions:", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestDir(d)

		zome, _ := h.GetZome("jsSampleZome")
		v, err := NewJSRibosome(h, zome)
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)

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

		// add entries onto the chain to get hash values for testing
		hash := commit(h, "oddNumbers", "3")
		profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)

		Convey("makeHash", func() {
			_, err = z.Run(`makeHash("3")`)
			So(err, ShouldBeNil)
			z := v.(*JSRibosome)
			hash1, err := NewHash(z.lastResult.String())
			So(err, ShouldBeNil)
			So(hash1.String(), ShouldEqual, hash.String())

			_, err = z.Run(`makeHash('{"firstName":"Zippy","lastName":"Pinhead"}')`)
			So(err, ShouldBeNil)
			hash1, err = NewHash(z.lastResult.String())
			So(err, ShouldBeNil)
			So(hash1.String(), ShouldEqual, profileHash.String())
		})
		Convey("call", func() {
			// a string calling function
			_, err := z.Run(`call("zySampleZome","addEven","432")`)
			So(err, ShouldBeNil)
			So(h.chain.Entries[len(h.chain.Hashes)-1].Content(), ShouldEqual, "432")
			z := v.(*JSRibosome)
			hash, _ := NewHash(z.lastResult.String())
			entry, _, _ := h.chain.GetEntry(hash)
			So(entry.Content(), ShouldEqual, "432")

			// a json calling function
			_, err = z.Run(`call("zySampleZome","addPrime",{prime:7})`)
			So(err, ShouldBeNil)
			So(h.chain.Entries[len(h.chain.Hashes)-1].Content(), ShouldEqual, `{"prime":7}`)
			hashJSONStr := z.lastResult.String()
			var hashStr string
			json.Unmarshal([]byte(hashJSONStr), &hashStr)
			hash, _ = NewHash(hashStr)
			entry, _, _ = h.chain.GetEntry(hash)
			So(entry.Content(), ShouldEqual, `{"prime":7}`)

		})
		Convey("send", func() {
			ShouldLog(h.nucleus.alog, `result was: "{\"pong\":\"foobar\"}"`, func() {
				_, err := z.Run(`debug("result was: "+JSON.stringify(send(App.Key.Hash,{ping:"foobar"})))`)
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestJSGenesis(t *testing.T) {
	Convey("it should fail if the init function returns false", t, func() {
		z, _ := NewJSRibosome(nil, &Zome{RibosomeType: JSRibosomeType, Code: `function genesis() {return false}`})
		err := z.ChainGenesis()
		So(err.Error(), ShouldEqual, "genesis failed")
	})
	Convey("it should work if the genesis function returns true", t, func() {
		z, _ := NewJSRibosome(nil, &Zome{RibosomeType: JSRibosomeType, Code: `function genesis() {return true}`})
		err := z.ChainGenesis()
		So(err, ShouldBeNil)
	})
}

func TestJSReceive(t *testing.T) {
	Convey("it should call a receive function", t, func() {
		z, _ := NewJSRibosome(nil, &Zome{RibosomeType: JSRibosomeType, Code: `function receive(from,msg) {return {foo:msg.bar}}`})
		response, err := z.Receive("fakehash", `{"bar":"baz"}`)
		So(err, ShouldBeNil)
		So(response, ShouldEqual, `{"foo":"baz"}`)
	})
}

func TestJSbuildValidate(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	e := GobEntry{C: "2"}
	a := NewCommitAction("evenNumbers", &e)
	var header Header
	a.header = &header

	def := EntryDef{Name: "evenNumbers", DataFormat: DataFormatString}

	Convey("it should build commit", t, func() {
		code, err := buildJSValidateAction(a, &def, nil, []string{"fake_src_hash"})
		So(err, ShouldBeNil)
		So(code, ShouldEqual, `validateCommit("evenNumbers","2",{"EntryLink":"","Type":"","Time":"0001-01-01T00:00:00Z"},{},["fake_src_hash"])`)
	})

	Convey("it should build put", t, func() {
		a := NewPutAction("evenNumbers", &e, &header)
		pkg, _ := MakePackage(h, PackagingReq{PkgReqChain: int64(PkgReqChainOptFull)})
		vpkg, _ := MakeValidationPackage(h, &pkg)
		_, err := buildJSValidateAction(a, &def, vpkg, []string{"fake_src_hash"})
		So(err, ShouldBeNil)
		//	So(code, ShouldEqual, `validatePut("evenNumbers","2",{"EntryLink":"","Type":"","Time":"0001-01-01T00:00:00Z"},pgk,["fake_src_hash"])`)
	})

}

func TestJSValidateCommit(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)
	//	a, _ := NewAgent(LibP2P, "Joe")
	//	h := NewHolochain(a, "some/path", "yaml", Zome{RibosomeType:JSRibosomeType,})
	//	a := h.agent
	h.config.Loggers.App.Format = ""
	h.config.Loggers.App.New(nil)
	hdr := mkTestHeader("evenNumbers")
	pkg, _ := MakePackage(h, PackagingReq{PkgReqChain: int64(PkgReqChainOptFull)})
	vpkg, _ := MakeValidationPackage(h, &pkg)

	Convey("it should be passing in the correct values", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function validateCommit(name,entry,header,pkg,sources) {debug(name);debug(entry);debug(JSON.stringify(header));debug(JSON.stringify(sources));debug(JSON.stringify(pkg));return true};`})
		So(err, ShouldBeNil)
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatString}
		ShouldLog(&h.config.Loggers.App, `evenNumbers
foo
{"EntryLink":"QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuU2","Time":"1970-01-01T00:00:01Z","Type":"evenNumbers"}
["fakehashvalue"]
{}
`, func() {
			a := NewCommitAction("oddNumbers", &GobEntry{C: "foo"})
			a.header = &hdr
			err = v.ValidateAction(a, &d, nil, []string{"fakehashvalue"})
			So(err, ShouldBeNil)
		})
	})
	Convey("should run an entry value against the defined validator for string data", t, func() {
		v, err := NewJSRibosome(nil, &Zome{RibosomeType: JSRibosomeType, Code: `function validateCommit(name,entry,header,pkg,sources) { return (entry=="fish")};`})
		So(err, ShouldBeNil)
		d := EntryDef{Name: "oddNumbers", DataFormat: DataFormatString}

		a := NewCommitAction("oddNumbers", &GobEntry{C: "cow"})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		a = NewCommitAction("oddNumbers", &GobEntry{C: "fish"})
		a.header = &hdr
		err = v.ValidateAction(a, &d, vpkg, nil)
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for js data", t, func() {
		v, err := NewJSRibosome(nil, &Zome{RibosomeType: JSRibosomeType, Code: `function validateCommit(name,entry,header,pkg,sources) { return (entry=="fish")};`})
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatRawJS}

		a := NewCommitAction("oddNumbers", &GobEntry{C: "\"cow\""})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		a = NewCommitAction("oddNumbers", &GobEntry{C: "\"fish\""})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil, nil)
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for json data", t, func() {
		v, err := NewJSRibosome(nil, &Zome{RibosomeType: JSRibosomeType, Code: `function validateCommit(name,entry,header,pkg,sources) { return (entry.data=="fish")};`})
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatJSON}

		a := NewCommitAction("evenNumbers", &GobEntry{C: `{"data":"cow"}`})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		a = NewCommitAction("evenNumbers", &GobEntry{C: `{"data":"fish"}`})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil, nil)
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
	Convey("it should prepare args for mod", t, func() {
		e := GobEntry{C: "4"}
		var header = Header{Type: "foo"}
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2") // fake hash for previous entry
		a := NewModAction("evenNumbers", &e, hash)
		a.header = &header

		args, err := prepareJSValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"4",{"EntryLink":"","Type":"foo","Time":"0001-01-01T00:00:00Z"},"QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2"`)
	})
	Convey("it should prepare args for del", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		entry := DelEntry{Hash: hash, Message: "expired"}
		a := NewDelAction("profile", entry)
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
		So(args, ShouldEqual, `"QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2",JSON.parse("[{\"LinkAction\":\"\",\"Base\":\"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5\",\"Link\":\"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5\",\"Tag\":\"fish\"}]")`)
	})
}

func TestJSSanitize(t *testing.T) {
	Convey("should strip quotes and returns", t, func() {
		So(jsSanitizeString(`"`), ShouldEqual, `\"`)
		So(jsSanitizeString("\"x\ny"), ShouldEqual, "\\\"xy")
	})
}

func TestJSExposeCall(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	v, zome, err := h.MakeRibosome("jsSampleZome")
	if err != nil {
		panic(err)
	}
	z := v.(*JSRibosome)
	Convey("should allow calling exposed STRING based functions", t, func() {
		cater, _ := zome.GetFunctionDef("testStrFn1")
		result, err := z.Call(cater, "fish \"zippy\"")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "result: fish \"zippy\"")

		adder, _ := zome.GetFunctionDef("testStrFn2")
		result, err = z.Call(adder, "10")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "12")
	})
	Convey("should allow calling exposed JSON based functions", t, func() {
		times2, _ := zome.GetFunctionDef("testJsonFn1")
		result, err := z.Call(times2, `{"input": 2}`)
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, `{"input":2,"output":4}`)
	})
	Convey("should sanitize against bad strings", t, func() {
		cater, _ := zome.GetFunctionDef("testStrFn1")
		result, err := z.Call(cater, "fish \"\nzippy\"")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "result: fish \"zippy\"")
	})
	Convey("should sanitize against bad JSON", t, func() {
		times2, _ := zome.GetFunctionDef("testJsonFn1")
		result, err := z.Call(times2, "{\"input\n\": 2}")
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, `{"input":2,"output":4}`)
	})
	Convey("should allow a function declared with JSON parameter to be called with no parameter", t, func() {
		emptyParametersJson, _ := zome.GetFunctionDef("testJsonFn2")
		result, err := z.Call(emptyParametersJson, "")
		So(err, ShouldBeNil)
		So(result, ShouldEqual, "[{\"a\":\"b\"}]")
	})
}

func TestJSDHT(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
	Convey("get should return hash not found if it doesn't exist", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s");`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		So(z.lastResult.String(), ShouldEqual, "HolochainError: hash not found")
	})

	// add an entry onto the chain
	hash = commit(h, "oddNumbers", "7")

	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}

	Convey("get should return entry", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s");`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", x.(Entry).Content()), ShouldEqual, `7`)
	})

	Convey("get should return entry type", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s",{GetMask:HC.GetMask.EntryType});`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", x.(string)), ShouldEqual, `oddNumbers`)
	})

	Convey("get should return sources", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s",{GetMask:HC.GetMask.Sources});`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", x), ShouldEqual, fmt.Sprintf("[%v]", h.nodeIDStr))
	})

	Convey("get should return collection", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s",{GetMask:HC.GetMask.All});`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		obj := x.(map[string]interface{})
		So(obj["Entry"].(Entry).Content(), ShouldEqual, `7`)
		So(obj["EntryType"].(string), ShouldEqual, `oddNumbers`)
		So(fmt.Sprintf("%v", obj["Sources"]), ShouldEqual, fmt.Sprintf("[%v]", h.nodeIDStr))
	})

	profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)

	commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String()))

	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}

	Convey("getLink function should return the Links", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`getLink("%s","4stars");`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		So(z.lastResult.Class(), ShouldEqual, "Object")
		x, err := z.lastResult.Export()
		lqr := x.(*LinkQueryResp)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", lqr.Links[0].H), ShouldEqual, profileHash.String())
	})

	Convey("getLink function with load option should return the Links and entries", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`getLink("%s","4stars",{Load:true});`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		So(z.lastResult.Class(), ShouldEqual, "Object")
		x, err := z.lastResult.Export()
		lqr := x.(*LinkQueryResp)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", lqr.Links[0].H), ShouldEqual, profileHash.String())
		So(fmt.Sprintf("%v", lqr.Links[0].E), ShouldEqual, `{"firstName":"Zippy","lastName":"Pinhead"}`)
	})

	Convey("commit with del link should delete link", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`commit("rating",{Links:[{"LinkAction":HC.LinkAction.Del,Base:"%s",Link:"%s",Tag:"4stars"}]});`, hash.String(), profileHash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		_, err = NewHash(z.lastResult.String())
		So(err, ShouldBeNil)

		if err := h.dht.simHandleChangeReqs(); err != nil {
			panic(err)
		}
		links, _ := h.dht.getLink(hash, "4stars", StatusLive)
		So(fmt.Sprintf("%v", links), ShouldEqual, "[]")
		links, _ = h.dht.getLink(hash, "4stars", StatusDeleted)
		So(fmt.Sprintf("%v", links), ShouldEqual, "[{QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt }]")
	})

	Convey("getLink function with StatusMask option should return deleted Links", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`getLink("%s","4stars",{StatusMask:HC.Status.Deleted});`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)

		So(z.lastResult.Class(), ShouldEqual, "Object")
		x, err := z.lastResult.Export()
		lqr := x.(*LinkQueryResp)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", lqr.Links[0].H), ShouldEqual, profileHash.String())
	})

	Convey("update function should commit a new entry and on DHT mark item modified", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`update("profile",{firstName:"Zippy",lastName:"ThePinhead"},"%s")`, profileHash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		profileHashStr2 := z.lastResult.String()

		header := h.chain.Top()
		So(header.EntryLink.String(), ShouldEqual, profileHashStr2)
		So(header.Change.Action, ShouldEqual, ModAction)
		So(header.Change.Hash.String(), ShouldEqual, profileHash.String())

		if err := h.dht.simHandleChangeReqs(); err != nil {
			panic(err)
		}

		// the entry should be marked as Modifed
		data, _, _, _, err := h.dht.get(profileHash, StatusDefault, GetMaskDefault)
		So(err, ShouldEqual, ErrHashModified)
		So(string(data), ShouldEqual, profileHashStr2)

		// but a regular get, should resolve through
		v, err = NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s");`, profileHash.String())})
		z = v.(*JSRibosome)
		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", x.(Entry).Content()), ShouldEqual, `{"firstName":"Zippy","lastName":"ThePinhead"}`)
	})

	Convey("remove function should mark item deleted", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`remove("%s","expired");`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		delhashstr := z.lastResult.String()
		_, err = NewHash(delhashstr)
		So(err, ShouldBeNil)

		v, err = NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s");`, hash.String())})
		So(err, ShouldBeNil)
		z = v.(*JSRibosome)
		So(z.lastResult.String(), ShouldEqual, "HolochainError: hash deleted")

		v, err = NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s",{StatusMask:HC.Status.Deleted});`, hash.String())})
		So(err, ShouldBeNil)
		z = v.(*JSRibosome)

		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", x.(Entry).Content()), ShouldEqual, `7`)
	})

	Convey("updateAgent function without options should fail", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType,
			Code: fmt.Sprintf(`updateAgent({})`)})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		x := z.lastResult.String()
		So(x, ShouldEqual, "HolochainError: expecting identity and/or revocation option")
	})

	Convey("updateAgent function should commit a new agent entry", t, func() {
		oldPubKey, _ := h.agent.PubKey().Bytes()
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType,
			Code: fmt.Sprintf(`updateAgent({Identity:"new identity"})`)})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		newAgentHash := z.lastResult.String()
		So(h.agentHash.String(), ShouldEqual, newAgentHash)
		header := h.chain.Top()
		So(header.Type, ShouldEqual, AgentEntryType)
		So(newAgentHash, ShouldEqual, header.EntryLink.String())
		So(h.agent.Identity(), ShouldEqual, "new identity")
		newPubKey, _ := h.agent.PubKey().Bytes()
		So(fmt.Sprintf("%v", newPubKey), ShouldEqual, fmt.Sprintf("%v", oldPubKey))
		entry, _, _ := h.chain.GetEntry(header.EntryLink)
		So(entry.Content().(AgentEntry).Identity, ShouldEqual, "new identity")
		So(fmt.Sprintf("%v", entry.Content().(AgentEntry).Key), ShouldEqual, fmt.Sprintf("%v", oldPubKey))
	})

	Convey("updateAgent function with revoke option should commit a new agent entry and mark key as modified on DHT", t, func() {
		oldPubKey, _ := h.agent.PubKey().Bytes()
		oldKey, _ := NewHash(h.nodeIDStr)
		oldAgentHash := h.agentHash

		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType,
			Code: fmt.Sprintf(`updateAgent({Revocation:"some revocation data"})`)})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		newAgentHash := z.lastResult.String()
		So(newAgentHash, ShouldEqual, h.agentHash.String())
		So(oldAgentHash.String(), ShouldNotEqual, h.agentHash.String())

		header := h.chain.Top()
		So(header.Type, ShouldEqual, AgentEntryType)
		So(newAgentHash, ShouldEqual, header.EntryLink.String())
		newPubKey, _ := h.agent.PubKey().Bytes()
		So(fmt.Sprintf("%v", newPubKey), ShouldNotEqual, fmt.Sprintf("%v", oldPubKey))
		entry, _, _ := h.chain.GetEntry(header.EntryLink)
		So(entry.Content().(AgentEntry).Revocation, ShouldEqual, "some revocation data")
		So(fmt.Sprintf("%v", entry.Content().(AgentEntry).Key), ShouldEqual, fmt.Sprintf("%v", newPubKey))

		// the new Key should be available on the DHT
		newKey, _ := NewHash(h.nodeIDStr)
		data, _, _, _, err := h.dht.get(newKey, StatusDefault, GetMaskDefault)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, string(newPubKey))

		// the old key should be marked as Modifed and we should get the new hash as the data
		data, _, _, _, err = h.dht.get(oldKey, StatusDefault, GetMaskDefault)
		So(err, ShouldEqual, ErrHashModified)
		So(string(data), ShouldEqual, h.nodeIDStr)

	})
}

func TestJSProcessArgs(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	v, _ := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: ""})
	z := v.(*JSRibosome)

	nilValue := otto.UndefinedValue()
	Convey("it should check for wrong number of args", t, func() {
		oArgs := []otto.Value{nilValue, nilValue}
		args := []Arg{{}}
		err := jsProcessArgs(z, args, oArgs)
		So(err, ShouldEqual, ErrWrongNargs)

		// test with args that are optional: two that are required and one not
		args = []Arg{{}, {}, {Optional: true}}
		oArgs = []otto.Value{nilValue}
		err = jsProcessArgs(z, args, oArgs)
		So(err, ShouldEqual, ErrWrongNargs)

		oArgs = []otto.Value{nilValue, nilValue, nilValue, nilValue}
		err = jsProcessArgs(z, args, oArgs)
		So(err, ShouldEqual, ErrWrongNargs)

	})
	Convey("it should treat StringArg as string", t, func() {
		args := []Arg{{Name: "foo", Type: StringArg}}
		err := jsProcessArgs(z, args, []otto.Value{nilValue})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string")
		val, _ := z.vm.ToValue("bar")
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")
	})
	Convey("it should convert IntArg to int64", t, func() {
		args := []Arg{{Name: "foo", Type: IntArg}}
		err := jsProcessArgs(z, args, []otto.Value{nilValue})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be int")
		val, _ := z.vm.ToValue(314)
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(int64), ShouldEqual, 314)
	})
	Convey("it should convert BoolArg to bool", t, func() {
		args := []Arg{{Name: "foo", Type: BoolArg}}
		err := jsProcessArgs(z, args, []otto.Value{nilValue})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be boolean")
		val, _ := z.vm.ToValue(true)
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(bool), ShouldEqual, true)
	})

	Convey("it should convert EntryArg from string or object", t, func() {
		args := []Arg{{Name: "foo", Type: EntryArg}}
		err := jsProcessArgs(z, args, []otto.Value{nilValue})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string or object")
		val, _ := z.vm.ToValue("bar")
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")

		// create an otto.object for a test arg
		val, _ = z.vm.ToValue(map[string]string{"H": "foo", "E": "bar"})
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, `{"E":"bar","H":"foo"}`)
	})

	// currently ArgsArg and EntryArg are identical, but we expect this to change
	Convey("it should convert ArgsArg from string or object", t, func() {
		args := []Arg{{Name: "foo", Type: ArgsArg}}
		err := jsProcessArgs(z, args, []otto.Value{nilValue})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string or object")
		val, _ := z.vm.ToValue("bar")
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")

		// create an otto.object for a test arg
		val, _ = z.vm.ToValue(map[string]string{"H": "foo", "E": "bar"})
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, `{"E":"bar","H":"foo"}`)
	})

	Convey("it should convert MapArg a map", t, func() {
		args := []Arg{{Name: "foo", Type: MapArg}}
		err := jsProcessArgs(z, args, []otto.Value{nilValue})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be object")

		// create a js object
		m := make(map[string]interface{})
		m["H"] = "fakehashvalue"
		m["I"] = 314
		val, _ := z.vm.ToValue(m)
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		x := args[0].value.(map[string]interface{})
		So(x["H"].(string), ShouldEqual, "fakehashvalue")
		So(x["I"].(int), ShouldEqual, 314)
	})

	Convey("it should convert ToStrArg any type to a string", t, func() {
		args := []Arg{{Name: "any", Type: ToStrArg}}
		val, _ := z.vm.ToValue("bar")
		err := jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")
		val, _ = z.vm.ToValue(123)
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "123")
		val, _ = z.vm.ToValue(true)
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "true")
		m := make(map[string]interface{})
		m["H"] = "fakehashvalue"
		m["I"] = 314
		val, _ = z.vm.ToValue(m)
		err = jsProcessArgs(z, args, []otto.Value{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, `{"H":"fakehashvalue","I":314}`)

	})
}
