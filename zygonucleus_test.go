package holochain

import (
	"fmt"
	zygo "github.com/glycerine/zygomys/repl"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
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

	Convey("it should have an App structure:", t, func() {
		d, _, h := prepareTestChain("test")
		defer cleanupTestDir(d)

		v, err := NewZygoNucleus(h, "")
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)

		_, err = z.Run("App_Name")
		So(err, ShouldBeNil)
		s := z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, h.Name)
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
		So(s, ShouldEqual, h.Agent().Name())

		_, err = z.Run("App_Key_Hash")
		So(err, ShouldBeNil)
		s = z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, peer.IDB58Encode(h.id))
	})

	Convey("it should have an HC structure:", t, func() {
		d, _, h := prepareTestChain("test")
		defer cleanupTestDir(d)

		v, err := NewZygoNucleus(h, "")
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)

		_, err = z.Run("HC_Version")
		So(err, ShouldBeNil)
		s := z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, VersionStr)

		_, err = z.Run("HC_StatusDeleted")
		So(err, ShouldBeNil)
		i := z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusDeleted)

		_, err = z.Run("HC_StatusLive")
		So(err, ShouldBeNil)
		i = z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusLive)

		_, err = z.Run("HC_StatusRejected")
		So(err, ShouldBeNil)
		i = z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusRejected)

		_, err = z.Run("HC_StatusModified")
		So(err, ShouldBeNil)
		i = z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusModified)

		_, err = z.Run("HC_StatusAny")
		So(err, ShouldBeNil)
		i = z.lastResult.(*zygo.SexpInt).Val
		So(i, ShouldEqual, StatusAny)
	})

	Convey("should have the built in functions:", t, func() {
		d, _, h := prepareTestChain("test")
		defer cleanupTestDir(d)

		v, err := NewZygoNucleus(h, "")
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)

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
	})
}

func TestZygoGenesis(t *testing.T) {
	Convey("it should fail if the genesis function returns false", t, func() {
		z, _ := NewZygoNucleus(nil, `(defn genesis [] false)`)
		err := z.ChainGenesis()
		So(err.Error(), ShouldEqual, "genesis failed")
	})
	Convey("it should work if the genesis function returns true", t, func() {
		z, _ := NewZygoNucleus(nil, `(defn genesis [] true)`)
		err := z.ChainGenesis()
		So(err, ShouldBeNil)
	})
}

func TestZybuildValidate(t *testing.T) {
	e := GobEntry{C: "3"}
	a := NewCommitAction("oddNumbers", &e)
	var header Header
	a.header = &header

	d := EntryDef{Name: "oddNumbers", DataFormat: DataFormatString}

	Convey("it should build commit", t, func() {
		code, err := buildZyValidateAction(a, &d, []string{"fake_src_hash"})
		So(err, ShouldBeNil)
		So(code, ShouldEqual, `(validateCommit "oddNumbers" "3" (hash EntryLink:"" Type:"" Time:"0001-01-01T00:00:00Z") (unjson (raw "[\"fake_src_hash\"]")))`)
	})
}

func TestZyValidateCommit(t *testing.T) {
	a, _ := NewAgent(IPFS, "Joe")
	h := NewHolochain(a, "some/path", "yaml", Zome{})
	h.config.Loggers.App.New(nil)
	hdr := mkTestHeader("evenNumbers")

	Convey("it should be passing in the correct values", t, func() {
		v, err := NewZygoNucleus(&h, `(defn validateCommit [name entry header sources] (debug name) (debug entry) (debug header) (debug sources) true)`)
		So(err, ShouldBeNil)
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatString}
		ShouldLog(&h.config.Loggers.App, `evenNumbers
foo
{"Atype":"hash", "EntryLink":"QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuU2", "Type":"evenNumbers", "Time":"1970-01-01T00:00:01Z", "zKeyOrder":["EntryLink", "Type", "Time"]}
["fakehashvalue"]
`, func() {
			a := NewCommitAction("oddNumbers", &GobEntry{C: "foo"})
			a.header = &hdr
			err = v.ValidateAction(a, &d, []string{"fakehashvalue"})
			So(err, ShouldBeNil)
		})
	})
	Convey("should run an entry value against the defined validator for string data", t, func() {
		v, err := NewZygoNucleus(nil, `(defn validateCommit [name entry header sources] (cond (== entry "fish") true false))`)
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
	Convey("should run an entry value against the defined validator for zygo data", t, func() {
		v, err := NewZygoNucleus(nil, `(defn validateCommit [name entry header sources] (cond (== entry "fish") true false))`)
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatRawZygo}

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
		v, err := NewZygoNucleus(nil, `(defn validateCommit [name entry header sources] (cond (== (hget entry data:) "fish") true false))`)
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
	Convey("it should prepare args for del", t, func() {
		hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
		a := NewDelAction(hash)
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
		So(args, ShouldEqual, `"QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2" (unjson (raw "[{\"Base\":\"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5\",\"Link\":\"QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5\",\"Tag\":\"fish\"}]"))`)
	})
}

func TestZySanitize(t *testing.T) {
	Convey("should strip quotes", t, func() {
		So(sanitizeZyString(`"`), ShouldEqual, `\"`)
		So(sanitizeZyString("\"x\ny"), ShouldEqual, "\\\"x\ny")
	})
}

func TestZygoExposeCall(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	zome, _ := h.GetZome("zySampleZome")
	v, err := h.makeNucleus(zome)
	if err != nil {
		panic(err)
	}
	z := v.(*ZygoNucleus)

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
		So(string(result.([]byte)), ShouldEqual, `{"Atype":"hash", "input":2, "output":4, "zKeyOrder":["input", "output"]}`)
	})
	Convey("should allow a function declared with JSON parameter to be called with no parameter", t, func() {
		emptyParametersJson, _ := h.GetFunctionDef(zome, "testJsonFn2")
		result, err := z.Call(emptyParametersJson, "")
		So(err, ShouldBeNil)
		So(string(result.([]byte)), ShouldEqual, "[{\"Atype\":\"hash\", \"a\":\"b\", \"zKeyOrder\":[\"a\"]}]")
	})
}

func TestZygoDHT(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
	Convey("get should return hash not found if it doesn't exist", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(get "%s")`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
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
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(get "%s")`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `"2"`)
	})

	profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)

	commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String()))

	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}

	Convey("getLink function should return the Links", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(getLink "%s" "4stars")`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
		sh := z.lastResult.(*zygo.SexpHash)

		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `[{"H":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","E":""}]`)
	})
	Convey("getLink function with load option should return the Links and entries", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(getLink "%s" "4stars" (hash Load:true))`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
		sh := z.lastResult.(*zygo.SexpHash)

		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `[{"H":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","E":"{\"firstName\":\"Zippy\",\"lastName\":\"Pinhead\"}"}]`)
	})

	Convey("delLink function should delete link", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(delLink "%s" "%s" "4stars")`, hash.String(), profileHash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)
		sh := z.lastResult.(*zygo.SexpHash)
		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(fmt.Sprintf("%v", r), ShouldEqual, "&{0}")
		links, _ := h.dht.getLink(hash, "4stars", StatusLive)
		So(fmt.Sprintf("%v", links), ShouldEqual, "[]")
		links, _ = h.dht.getLink(hash, "4stars", StatusDeleted)
		So(fmt.Sprintf("%v", links), ShouldEqual, "[{QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt }]")
	})

	Convey("getLink function with StatusMask option should return deleted Links", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(getLink "%s" "4stars" (hash StatusMask:HC_StatusDeleted))`, hash.String()))
		So(err, ShouldBeNil)
		z := v.(*ZygoNucleus)

		sh := z.lastResult.(*zygo.SexpHash)
		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `[{"H":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","E":""}]`)
	})

	Convey("del function should mark item deleted", t, func() {
		v, err := NewZygoNucleus(h, fmt.Sprintf(`(del "%s")`, hash.String()))
		So(err, ShouldBeNil)

		z := v.(*ZygoNucleus)
		sh := z.lastResult.(*zygo.SexpHash)
		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(fmt.Sprintf("%v", r), ShouldEqual, "&{0}")

		v, err = NewZygoNucleus(h, fmt.Sprintf(`(get "%s")`, hash.String()))
		So(err, ShouldBeNil)
		z = v.(*ZygoNucleus)

		r, err = z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("error"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, "hash not found")

		v, err = NewZygoNucleus(h, fmt.Sprintf(`(get "%s" HC_StatusDeleted)`, hash.String()))
		So(err, ShouldBeNil)
		z = v.(*ZygoNucleus)

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

	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("it should convert EntryArg from string or hash", t, func() {
		args := []Arg{{Name: "foo", Type: EntryArg}}
		err := zyProcessArgs(args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string or hash")
		var val zygo.Sexp = &zygo.SexpStr{S: "bar"}
		err = zyProcessArgs(args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")

		// create a zygo hash for a test arg

		v, err := NewZygoNucleus(h, "")
		env := v.(*ZygoNucleus).env
		hval, _ := zygo.MakeHash(nil, "hash", env)
		hval.HashSet(env.MakeSymbol("fname"), &zygo.SexpStr{S: "Jane"})
		hval.HashSet(env.MakeSymbol("lname"), &zygo.SexpStr{S: "Smith"})
		err = zyProcessArgs(args, []zygo.Sexp{hval})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, `{"Atype":"hash", "fname":"Jane", "lname":"Smith", "zKeyOrder":["fname", "lname"]}`)

	})

	Convey("it should convert MapArg to a map", t, func() {
		args := []Arg{{Name: "foo", Type: MapArg}}
		err := zyProcessArgs(args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be hash")

		// create a zygo hash as a test arg
		v, err := NewZygoNucleus(h, "")
		env := v.(*ZygoNucleus).env
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
		v, err := NewZygoNucleus(h, "")
		env := v.(*ZygoNucleus).env
		var hashstr zygo.Sexp = &zygo.SexpStr{S: "fakehashvalue"}
		hval, _ := zygo.MakeHash(nil, "hash", env)
		hval.HashSet(env.MakeSymbol("H"), hashstr)
		hval.HashSet(env.MakeSymbol("I"), &zygo.SexpInt{Val: 314})

		err = zyProcessArgs(args, []zygo.Sexp{hval})
		So(args[0].value.(string), ShouldEqual, `{"Atype":"hash", "H":"fakehashvalue", "I":314, "zKeyOrder":["H", "I"]}`)

	})
}
