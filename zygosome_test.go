package holochain

import (
	"encoding/json"
	"fmt"
	zygo "github.com/glycerine/zygomys/repl"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestNewZygoRibosome(t *testing.T) {
	Convey("new should create a ribosome", t, func() {
		v, err := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(+ 1 1)`})
		z := v.(*ZygoRibosome)
		So(err, ShouldBeNil)
		So(z.lastResult.(*zygo.SexpInt).Val, ShouldEqual, 2)
	})
	Convey("new fail to create ribosome when code is bad", t, func() {
		v, err := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: "(should make a zygo syntax error"})
		So(v, ShouldBeNil)
		So(err.Error(), ShouldEqual, "Zygomys load error: Error on line 1: parser needs more input\n")
	})

	Convey("it should have an App structure:", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestDir(d)

		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)

		_, err = z.Run("App_Name")
		So(err, ShouldBeNil)
		s := z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, h.nucleus.dna.Name)
		_, err = z.Run("App_DNA_Hash")
		So(err, ShouldBeNil)
		s = z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, h.dnaHash.String())
		_, err = z.Run("App_Agent_Hash")
		So(err, ShouldBeNil)
		s = z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, h.agentHash.String())

		_, err = z.Run("App_Agent_String")
		So(err, ShouldBeNil)
		s = z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, h.Agent().Identity())

		_, err = z.Run("App_Key_Hash")
		So(err, ShouldBeNil)
		s = z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, h.nodeIDStr)
	})

	Convey("it should have an HC structure:", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestDir(d)

		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)

		_, err = z.Run("HC_Version")
		So(err, ShouldBeNil)
		s := z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, VersionStr)

		_, err = z.Run("HC_Status_Deleted")
		So(err, ShouldBeNil)
		i := z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusDeleted)

		_, err = z.Run("HC_Status_Live")
		So(err, ShouldBeNil)
		i = z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusLive)

		_, err = z.Run("HC_Status_Rejected")
		So(err, ShouldBeNil)
		i = z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusRejected)

		_, err = z.Run("HC_Status_Modified")
		So(err, ShouldBeNil)
		i = z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusModified)

		_, err = z.Run("HC_Status_Any")
		So(err, ShouldBeNil)
		i = z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusAny)
	})

	Convey("should have the built in functions:", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestDir(d)

		zome, _ := h.GetZome("zySampleZome")
		v, err := NewZygoRibosome(h, zome)
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)

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

			ShouldLog(&infoLog, "Warning: Getting special properties via property() is deprecated as of 3. Returning nil values.  Use App* instead\n", func() {
				_, err = z.Run(`(property "` + ID_PROPERTY + `")`)
				So(err, ShouldBeNil)
			})

		})

		// add entries onto the chain to get hash values for testing
		hash := commit(h, "oddNumbers", "3")
		profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)

		Convey("makeHash", func() {
			_, err = z.Run(`(makeHash "3")`)
			So(err, ShouldBeNil)
			z := v.(*ZygoRibosome)
			hash1, err := NewHash(z.lastResult.(*zygo.SexpStr).S)
			So(err, ShouldBeNil)
			So(hash1.String(), ShouldEqual, hash.String())

			_, err = z.Run(`(makeHash "{\"firstName\":\"Zippy\",\"lastName\":\"Pinhead\"}")`)
			So(err, ShouldBeNil)
			hash1, err = NewHash(z.lastResult.(*zygo.SexpStr).S)
			So(err, ShouldBeNil)
			So(hash1.String(), ShouldEqual, profileHash.String())
		})

		Convey("call", func() {
			// a string calling function
			_, err := z.Run(`(call "jsSampleZome" "addOdd" "321")`)
			So(err, ShouldBeNil)
			So(h.chain.Entries[len(h.chain.Hashes)-1].Content(), ShouldEqual, "321")
			z := v.(*ZygoRibosome)
			hashStr := z.lastResult.(*zygo.SexpStr).S
			hash, _ := NewHash(hashStr)
			entry, _, _ := h.chain.GetEntry(hash)
			So(entry.Content(), ShouldEqual, "321")

			// a json calling function
			_, err = z.Run(`(call "jsSampleZome" "addProfile" (hash firstName: "Jane" lastName: "Jetson"))`)
			So(err, ShouldBeNil)
			So(h.chain.Entries[len(h.chain.Hashes)-1].Content(), ShouldEqual, `{"firstName":"Jane","lastName":"Jetson"}`)
			hashJSONStr := z.lastResult.(*zygo.SexpStr).S
			json.Unmarshal([]byte(hashJSONStr), &hashStr)
			hash, _ = NewHash(hashStr)
			entry, _, _ = h.chain.GetEntry(hash)
			So(entry.Content(), ShouldEqual, `{"firstName":"Jane","lastName":"Jetson"}`)

		})
		Convey("send", func() {
			ShouldLog(h.nucleus.alog, `result was: "{\"pong\":\"foobar\"}"`, func() {
				_, err := z.Run(`(debug (concat "result was: " (str (hget (send App_Key_Hash (hash ping: "foobar")) %result))))`)
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestZygoGenesis(t *testing.T) {
	Convey("it should fail if the genesis function returns false", t, func() {
		z, _ := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn genesis [] false)`})
		err := z.ChainGenesis()
		So(err.Error(), ShouldEqual, "genesis failed")
	})
	Convey("it should work if the genesis function returns true", t, func() {
		z, _ := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn genesis [] true)`})
		err := z.ChainGenesis()
		So(err, ShouldBeNil)
	})
}

func TestZyReceive(t *testing.T) {
	Convey("it should call a receive function that returns a hash", t, func() {
		z, _ := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn receive [from msg] (hash %foo (hget msg %bar)))`})
		response, err := z.Receive("fakehash", `{"bar":"baz"}`)
		So(err, ShouldBeNil)
		So(response, ShouldEqual, `{"foo":"baz"}`)
	})

	Convey("it should call a receive function that returns a string", t, func() {
		z, _ := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn receive [from msg] (concat "fish:" (hget msg %bar)))`})
		response, err := z.Receive("fakehash", `{"bar":"baz"}`)
		So(err, ShouldBeNil)
		So(response, ShouldEqual, `"fish:baz"`)
	})

	Convey("it should call a receive function that returns an int", t, func() {
		z, _ := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn receive [from msg] (len (hget msg %bar)))`})
		response, err := z.Receive("fakehash", `{"bar":"baz"}`)
		So(err, ShouldBeNil)
		So(response, ShouldEqual, `3`)
	})
}

func TestZybuildValidate(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	e := GobEntry{C: "3"}
	a := NewCommitAction("oddNumbers", &e)
	var header Header
	a.header = &header

	def := EntryDef{Name: "oddNumbers", DataFormat: DataFormatString}

	Convey("it should build commit", t, func() {
		code, err := buildZyValidateAction(a, &def, nil, []string{"fake_src_hash"})
		So(err, ShouldBeNil)
		So(code, ShouldEqual, `(validateCommit "oddNumbers" "3" (hash EntryLink:"" Type:"" Time:"0001-01-01T00:00:00Z") (hash) (unjson (raw "[\"fake_src_hash\"]")))`)
	})
	Convey("it should build put", t, func() {
		a := NewPutAction("evenNumbers", &e, &header)
		pkg, _ := MakePackage(h, PackagingReq{PkgReqChain: int64(PkgReqChainOptFull)})
		vpkg, _ := MakeValidationPackage(h, &pkg)
		_, err := buildZyValidateAction(a, &def, vpkg, []string{"fake_src_hash"})
		So(err, ShouldBeNil)
		//So(code, ShouldEqual, `validatePut("evenNumbers","2",{"EntryLink":"","Type":"","Time":"0001-01-01T00:00:00Z"},pgk,["fake_src_hash"])`)
	})
}

func TestZyValidateCommit(t *testing.T) {
	a, _ := NewAgent(LibP2P, "Joe")
	h := NewHolochain(a, "some/path", "yaml", Zome{RibosomeType: ZygoRibosomeType})
	h.config.Loggers.App.New(nil)
	hdr := mkTestHeader("evenNumbers")

	Convey("it should be passing in the correct values", t, func() {
		v, err := NewZygoRibosome(&h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn validateCommit [name entry header pkg sources] (debug name) (debug entry) (debug header) (debug sources) (debug pkg) true)`})
		So(err, ShouldBeNil)
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatString}
		ShouldLog(&h.config.Loggers.App, `evenNumbers
foo
{"EntryLink":"QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuU2", "Type":"evenNumbers", "Time":"1970-01-01T00:00:01Z"}
["fakehashvalue"]
{"Atype":"hash"}
`, func() {
			a := NewCommitAction("oddNumbers", &GobEntry{C: "foo"})
			a.header = &hdr
			err = v.ValidateAction(a, &d, nil, []string{"fakehashvalue"})
			So(err, ShouldBeNil)
		})
	})
	Convey("should run an entry value against the defined validator for string data", t, func() {
		v, err := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn validateCommit [name entry header pkg sources] (cond (== entry "fish") true false))`})
		So(err, ShouldBeNil)
		d := EntryDef{Name: "oddNumbers", DataFormat: DataFormatString}

		a := NewCommitAction("oddNumbers", &GobEntry{C: "cow"})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil, nil)
		So(err, ShouldEqual, ValidationFailedErr)

		a = NewCommitAction("oddNumbers", &GobEntry{C: "fish"})
		a.header = &hdr
		err = v.ValidateAction(a, &d, nil, nil)
		So(err, ShouldBeNil)
	})
	Convey("should run an entry value against the defined validator for zygo data", t, func() {
		v, err := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn validateCommit [name entry header pkg sources] (cond (== entry "fish") true false))`})
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatRawZygo}

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
		v, err := NewZygoRibosome(nil, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn validateCommit [name entry header pkg sources] (cond (== (hget entry data:) "fish") true false))`})
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

func TestPrepareZyValidateArgs(t *testing.T) {
	d := EntryDef{Name: "oddNumbers", DataFormat: DataFormatString}

	Convey("it should prepare args for commit", t, func() {
		e := GobEntry{C: "3"}
		a := NewCommitAction("oddNumbers", &e)
		var header Header
		a.header = &header
		args, err := prepareZyValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"3" (hash EntryLink:"" Type:"" Time:"0001-01-01T00:00:00Z")`)
	})
	Convey("it should prepare args for put", t, func() {
		e := GobEntry{C: "3"}
		var header Header
		a := NewPutAction("oddNumbers", &e, &header)

		args, err := prepareZyValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"3" (hash EntryLink:"" Type:"" Time:"0001-01-01T00:00:00Z")`)
	})
	Convey("it should prepare args for mod", t, func() {
		e := GobEntry{C: "7"}
		var header = Header{Type: "foo"}
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2") // fake hash for previous
		a := NewModAction("oddNumbers", &e, hash)
		a.header = &header
		args, err := prepareZyValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"7" (hash EntryLink:"" Type:"foo" Time:"0001-01-01T00:00:00Z") "QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2"`)
	})
	Convey("it should prepare args for del", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		entry := DelEntry{Hash: hash, Message: "expired"}
		a := NewDelAction("profile", entry)
		args, err := prepareZyValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2"`)
	})
	Convey("it should prepare args for link", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		a := NewLinkAction("oddNumbers", []Link{{Base: "QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5", Link: "QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5", Tag: "fish"}})
		a.validationBase = hash
		args, err := prepareZyValidateArgs(a, &d)
		So(err, ShouldBeNil)
		So(args, ShouldEqual, `"QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2" (unjson (raw "[{\"LinkAction\":\"\",\"Base\":\"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5\",\"Link\":\"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5\",\"Tag\":\"fish\"}]"))`)
	})
}

func TestZySanitize(t *testing.T) {
	Convey("should strip quotes", t, func() {
		So(sanitizeZyString(`"`), ShouldEqual, `\"`)
		So(sanitizeZyString("\"x\ny"), ShouldEqual, "\\\"x\ny")
	})
}

func TestZygoExposeCall(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	v, zome, err := h.MakeRibosome("zySampleZome")
	if err != nil {
		panic(err)
	}
	z := v.(*ZygoRibosome)

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
		So(string(result.(string)), ShouldEqual, `{"input":2, "output":4}`)
	})
	Convey("should allow a function declared with JSON parameter to be called with no parameter", t, func() {
		emptyParametersJSON, _ := zome.GetFunctionDef("testJsonFn2")
		result, err := z.Call(emptyParametersJSON, "")
		So(err, ShouldBeNil)
		So(string(result.(string)), ShouldEqual, `[{"a":"b"}]`)
	})
}

func TestZygoDHT(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
	Convey("get should return hash not found if it doesn't exist", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(get "%s")`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("error"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, "hash not found")
	})
	// add an entry onto the chain
	hash = commit(h, "evenNumbers", "2")

	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}

	Convey("get should return entry", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(get "%s")`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `"2"`)
	})

	Convey("get should return entry type", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(get "%s" (hash GetMask:HC_GetMask_EntryType))`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, "evenNumbers")
	})

	Convey("get should return sources", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(get "%s" (hash GetMask:HC_GetMask_Sources))`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpArray).Val[0].(*zygo.SexpStr).S, ShouldEqual, h.nodeIDStr)
	})

	Convey("get should return collection", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(get "%s" (hash GetMask:HC_GetMask_All))`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		resp := r.(*zygo.SexpHash)
		e, _ := resp.HashGet(z.env, z.env.MakeSymbol("EntryType"))
		So(e.(*zygo.SexpStr).S, ShouldEqual, "evenNumbers")
		e, _ = resp.HashGet(z.env, z.env.MakeSymbol("Entry"))
		So(e.(*zygo.SexpStr).S, ShouldEqual, `"2"`)
		e, _ = resp.HashGet(z.env, z.env.MakeSymbol("Sources"))
		So(e.(*zygo.SexpArray).Val[0].(*zygo.SexpStr).S, ShouldEqual, h.nodeIDStr)
	})
	profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)
	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}

	commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String()))
	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}

	Convey("getLink function should return the Links", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(getLink "%s" "4stars")`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		sh := z.lastResult.(*zygo.SexpHash)

		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `[{"H":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","E":""}]`)
	})
	Convey("getLink function with load option should return the Links and entries", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(getLink "%s" "4stars" (hash Load:true))`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		sh := z.lastResult.(*zygo.SexpHash)

		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `[{"H":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","E":"{\"firstName\":\"Zippy\",\"lastName\":\"Pinhead\"}"}]`)
	})

	Convey("commit with del link should delete link", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(commit "rating" (hash Links:[(hash LinkAction:HC_LinkAction_Del Base:"%s" Link:"%s" Tag:"4stars")]))`, hash.String(), profileHash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)

		_, err = NewHash(z.lastResult.(*zygo.SexpStr).S)
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
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(getLink "%s" "4stars" (hash StatusMask:HC_Status_Deleted))`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)

		sh := z.lastResult.(*zygo.SexpHash)
		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `[{"H":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","E":""}]`)
	})

	Convey("update function should commit a new entry and on DHT mark item modified", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(update "profile" (hash firstName:"Zippy" lastName:"ThePinhead") "%s")`, profileHash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		profileHashStr2 := z.lastResult.(*zygo.SexpStr).S

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
		v, err = NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(get "%s")`, profileHash.String())})
		So(err, ShouldBeNil)
		z = v.(*ZygoRibosome)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `"{\"firstName\":\"Zippy\", \"lastName\":\"ThePinhead\"}"`)
	})

	Convey("remove function should mark item deleted", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(remove "%s" "expired")`, hash.String())})
		So(err, ShouldBeNil)

		z := v.(*ZygoRibosome)
		_, err = NewHash(z.lastResult.(*zygo.SexpStr).S)
		So(err, ShouldBeNil)

		v, err = NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(get "%s")`, hash.String())})
		So(err, ShouldBeNil)
		z = v.(*ZygoRibosome)

		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("error"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, "hash deleted")

		v, err = NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(get "%s" (hash StatusMask:HC_Status_Deleted))`, hash.String())})
		So(err, ShouldBeNil)
		z = v.(*ZygoRibosome)

		r, err = z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `"2"`)
	})

}

func TestZyProcessArgs(t *testing.T) {
	Convey("it should check for wrong number of args", t, func() {
		zyargs := []zygo.Sexp{zygo.SexpNull, zygo.SexpNull}
		args := []Arg{{}}
		err := zyProcessArgs(args, zyargs)
		So(err, ShouldEqual, ErrWrongNargs)

		// test with args that are optional: two that are required and one not
		args = []Arg{{}, {}, {Optional: true}}
		zyargs = []zygo.Sexp{zygo.SexpNull}
		err = zyProcessArgs(args, zyargs)
		So(err, ShouldEqual, ErrWrongNargs)

		zyargs = []zygo.Sexp{zygo.SexpNull, zygo.SexpNull, zygo.SexpNull, zygo.SexpNull}
		err = zyProcessArgs(args, zyargs)
		So(err, ShouldEqual, ErrWrongNargs)
	})
	Convey("it should convert HashArg to Hash", t, func() {
		hashstr := "QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2"
		args := []Arg{{Name: "foo", Type: HashArg}}
		err := zyProcessArgs(args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string")
		var val zygo.Sexp = &zygo.SexpStr{S: hashstr}
		err = zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(Hash).String(), ShouldEqual, hashstr)
	})
	Convey("it should treat StringArg as string", t, func() {
		args := []Arg{{Name: "foo", Type: StringArg}}
		err := zyProcessArgs(args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string")
		var val zygo.Sexp = &zygo.SexpStr{S: "bar"}
		err = zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")
	})
	Convey("it should convert IntArg to int64", t, func() {
		args := []Arg{{Name: "foo", Type: IntArg}}
		err := zyProcessArgs(args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be int")
		var val zygo.Sexp = &zygo.SexpInt{Val: 314}
		err = zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(int64), ShouldEqual, 314)
	})
	Convey("it should convert BoolArg to bool", t, func() {
		args := []Arg{{Name: "foo", Type: BoolArg}}
		err := zyProcessArgs(args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be boolean")
		var val zygo.Sexp = &zygo.SexpBool{Val: true}
		err = zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(bool), ShouldEqual, true)
	})

	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	Convey("it should convert EntryArg from string or hash", t, func() {
		args := []Arg{{Name: "foo", Type: EntryArg}}
		err := zyProcessArgs(args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string or hash")
		var val zygo.Sexp = &zygo.SexpStr{S: "bar"}
		err = zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")

		// create a zygo hash for a test arg

		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
		env := v.(*ZygoRibosome).env
		hval, _ := zygo.MakeHash(nil, "hash", env)
		hval.HashSet(env.MakeSymbol("fname"), &zygo.SexpStr{S: "Jane"})
		hval.HashSet(env.MakeSymbol("lname"), &zygo.SexpStr{S: "Smith"})
		err = zyProcessArgs(args, []zygo.Sexp{hval})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, `{"fname":"Jane", "lname":"Smith"}`)

	})

	// currently ArgsArg and EntryArg are identical, but we expect this to change
	Convey("it should convert ArgsArg from string or hash", t, func() {
		args := []Arg{{Name: "foo", Type: ArgsArg}}
		err := zyProcessArgs(args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string or hash")
		var val zygo.Sexp = &zygo.SexpStr{S: "bar"}
		err = zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")

		// create a zygo hash for a test arg

		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
		env := v.(*ZygoRibosome).env
		hval, _ := zygo.MakeHash(nil, "hash", env)
		hval.HashSet(env.MakeSymbol("fname"), &zygo.SexpStr{S: "Jane"})
		hval.HashSet(env.MakeSymbol("lname"), &zygo.SexpStr{S: "Smith"})
		err = zyProcessArgs(args, []zygo.Sexp{hval})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, `{"fname":"Jane", "lname":"Smith"}`)

	})

	Convey("it should convert MapArg to a map", t, func() {
		args := []Arg{{Name: "foo", Type: MapArg}}
		err := zyProcessArgs(args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be hash")

		// create a zygo hash as a test arg
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
		env := v.(*ZygoRibosome).env
		var hashstr zygo.Sexp = &zygo.SexpStr{S: "fakehashvalue"}
		hval, _ := zygo.MakeHash(nil, "hash", env)
		hval.HashSet(env.MakeSymbol("H"), hashstr)
		hval.HashSet(env.MakeSymbol("I"), &zygo.SexpInt{Val: 314})

		err = zyProcessArgs(args, []zygo.Sexp{hval})
		So(err, ShouldBeNil)
		x := args[0].value.(map[string]interface{})
		So(x["H"].(string), ShouldEqual, "fakehashvalue")
		So(x["I"].(float64), ShouldEqual, 314)
	})

	Convey("it should convert ToStrArg any type to a string", t, func() {
		args := []Arg{{Name: "any", Type: ToStrArg}}
		var val zygo.Sexp = &zygo.SexpStr{S: "bar"}
		err := zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")
		val = &zygo.SexpInt{Val: 123}
		err = zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "123")
		val = &zygo.SexpBool{Val: true}
		err = zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "true")

		// create a zygo hash as a test arg
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
		env := v.(*ZygoRibosome).env
		var hashstr zygo.Sexp = &zygo.SexpStr{S: "fakehashvalue"}
		hval, _ := zygo.MakeHash(nil, "hash", env)
		hval.HashSet(env.MakeSymbol("H"), hashstr)
		hval.HashSet(env.MakeSymbol("I"), &zygo.SexpInt{Val: 314})

		err = zyProcessArgs(args, []zygo.Sexp{hval})
		So(args[0].value.(string), ShouldEqual, `{"H":"fakehashvalue", "I":314}`)
	})
}
