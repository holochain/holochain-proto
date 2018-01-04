package holochain

import (
	"encoding/json"
	"fmt"
	zygo "github.com/glycerine/zygomys/zygo"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"strings"
	"testing"
)

func TestNewZygoRibosome(t *testing.T) {
	Convey("new should create a ribosome", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestChain(h, d)
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(+ 1 1)`})
		z := v.(*ZygoRibosome)
		So(err, ShouldBeNil)
		So(z.lastResult.(*zygo.SexpInt).Val, ShouldEqual, 2)
	})
	Convey("new fail to create ribosome when code is bad", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestChain(h, d)
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: "(should make a zygo syntax error"})
		So(v, ShouldBeNil)
		So(err.Error(), ShouldEqual, "Zygomys load error: Error on line 1: parser needs more input\n")
	})

	Convey("it should have an App structure:", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestChain(h, d)

		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)

		_, err = z.Run("App_Name")
		So(err, ShouldBeNil)
		s := z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, h.Name())
		_, err = z.Run("App_DNA_Hash")
		So(err, ShouldBeNil)
		s = z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, h.dnaHash.String())
		_, err = z.Run("App_Agent_Hash")
		So(err, ShouldBeNil)
		s = z.lastResult.(*zygo.SexpStr).S
		So(s, ShouldEqual, h.agentHash.String())
		_, err = z.Run("App_Agent_TopHash")
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
		defer CleanupTestChain(h, d)

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
		defer CleanupTestChain(h, d)

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
		hash := commit(h, "evenNumbers", "4")
		profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)

		Convey("makeHash", func() {
			_, err = z.Run(`(makeHash "evenNumbers" "4")`)
			So(err, ShouldBeNil)
			z := v.(*ZygoRibosome)
			hash1, err := NewHash(z.lastResult.(*zygo.SexpStr).S)
			So(err, ShouldBeNil)
			So(hash1.String(), ShouldEqual, hash.String())

			_, err = z.Run(`(makeHash "profile" (hash firstName:"Zippy" lastName:"Pinhead"))`)
			So(err, ShouldBeNil)
			hash1, err = NewHash(z.lastResult.(*zygo.SexpStr).S)
			So(err, ShouldBeNil)
			So(hash1.String(), ShouldEqual, profileHash.String())
		})

		Convey("getBridges", func() {
			_, err = z.Run(`(getBridges)`)
			So(err, ShouldBeNil)
			z := v.(*ZygoRibosome)

			So(len(z.lastResult.(*zygo.SexpArray).Val), ShouldEqual, 0)

			hFromHash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzfrom")
			var token string
			token, err = h.AddBridgeAsCallee(hFromHash, "")
			if err != nil {
				panic(err)
			}

			hToHash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqto")
			err = h.AddBridgeAsCaller(hToHash, token, "fakeurl", "")
			if err != nil {
				panic(err)
			}

			ShouldLog(h.nucleus.alog, fmt.Sprintf(`[ (hash Side:0 ToApp:"QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqto")  (hash Side:1 Token:"%s")]`, token), func() {
				_, err := z.Run(`(testGetBridges)`)
				So(err, ShouldBeNil)
			})

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
		Convey("bridge", func() {
			// hard to test because we need to fire up a separate app someplace else
			_, err := z.Run(`(bridge "QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHw" "jsSampleZome" "getProperty" "language")`)
			So(err.Error(), ShouldEqual, "Zygomys exec error: Error calling 'bridge': no active bridge")
		})

		Convey("send", func() {
			ShouldLog(h.nucleus.alog, `result was: "{\"pong\":\"foobar\"}"`, func() {
				_, err := z.Run(`(debug (concat "result was: " (str (hget (send App_Key_Hash (hash ping: "foobar")) %result))))`)
				So(err, ShouldBeNil)
			})
		})
		Convey("send async", func() {
			ShouldLog(h.nucleus.alog, `async result of message with 123 was: (hash pong:"foobar")`, func() {
				_, err := z.Run(`(send App_Key_Hash (hash ping: "foobar") (hash Callback: (hash Function: "asyncPing" ID:"123")))`)
				So(err, ShouldBeNil)
				err = <-h.asyncSends
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestZygoQuery(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	zome, _ := h.GetZome("zySampleZome")
	v, err := NewZygoRibosome(h, zome)
	if err != nil {
		panic(err)
	}
	z := v.(*ZygoRibosome)

	Convey("query", t, func() {
		// add entries onto the chain to get hash values for testing
		commit(h, "evenNumbers", "2")
		commit(h, "secret", "foo")
		commit(h, "evenNumbers", "4")
		commit(h, "secret", "bar")
		commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)

		ShouldLog(h.nucleus.alog, `["2" "4"]`, func() {
			_, err := z.Run(`(debug (str (query (hash Constrain: (hash EntryTypes: ["evenNumbers"])))))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `["QmQzp4h9pvLVJHUx6rFxxC4ViqgnznYqXvoa9HsJgACMmi" "QmS4bKx7zZt6qoX2om5M5ik3X2k4Fco2nFx82CDJ3iVKj2"]`, func() {
			_, err := z.Run(`(debug (str (query (hash Return: (hash Hashes: true) Constrain: (hash EntryTypes:["evenNumbers"])))))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `[ (hash Hash:"QmQzp4h9pvLVJHUx6rFxxC4ViqgnznYqXvoa9HsJgACMmi" Entry:"2")  (hash Hash:"QmS4bKx7zZt6qoX2om5M5ik3X2k4Fco2nFx82CDJ3iVKj2" Entry:"4")]`, func() {
			_, err := z.Run(`(debug (str (query ( hash Return: (hash Hashes:true Entries:true) Constrain: (hash EntryTypes: ["evenNumbers"])))))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `"Type":"evenNumbers","EntryLink":"QmQzp4h9pvLVJHUx6rFxxC4ViqgnznYqXvoa9HsJgACMmi","HeaderLink":"Qm`, func() {
			_, err := z.Run(`(debug (query (hash Return: (hash Headers:true Entries:true) Constrain: (hash EntryTypes: ["evenNumbers"]))))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `["foo","bar"]`, func() {
			_, err := z.Run(`(debug (query (hash Constrain: (hash EntryTypes: ["secret"]))))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `["{\"firstName\":\"Zippy\",\"lastName\":\"Pinhead\"}"]`, func() {
			_, err := z.Run(`(debug (str (query (hash Constrain: (hash EntryTypes: ["profile"])))))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `["{\"Identity\":\"Herbert \\u003ch@bert.com\\u003e\",\"Revocation\":null,\"PublicKey\":\"CAESIHLUfxjdoEfk8byjsBR+FXxYpYrFTviSBf2BbC0boylT\"}"]`, func() {
			_, err := z.Run(`(debug (str (query (hash Constrain: (hash EntryTypes: ["%agent"])))))`)
			So(err, ShouldBeNil)
		})
	})
}
func TestZygoGenesis(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	Convey("it should fail if the genesis function returns false", t, func() {
		z, _ := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn genesis [] false)`})
		err := z.ChainGenesis()
		So(err.Error(), ShouldEqual, "genesis failed")
	})
	Convey("it should work if the genesis function returns true", t, func() {
		z, _ := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn genesis [] true)`})
		err := z.ChainGenesis()
		So(err, ShouldBeNil)
	})
}

func TestZygoBridgeGenesis(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	fakeToApp, _ := NewHash("QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx")
	Convey("it should fail if the bridge genesis function returns false", t, func() {
		ShouldLog(&h.Config.Loggers.App, h.dnaHash.String()+" test data", func() {
			z, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn bridgeGenesis [side app data] (begin (debug (concat app " " data)) false))`})
			So(err, ShouldBeNil)
			err = z.BridgeGenesis(BridgeFrom, h.dnaHash, "test data")
			So(err.Error(), ShouldEqual, "bridgeGenesis failed")
		})
	})
	Convey("it should work if the bridge genesis function returns true", t, func() {
		ShouldLog(&h.Config.Loggers.App, fakeToApp.String()+" test data", func() {
			z, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn bridgeGenesis [side app data] (begin (debug (concat app " " data)) true))`})
			So(err, ShouldBeNil)
			err = z.BridgeGenesis(BridgeTo, fakeToApp, "test data")
			So(err, ShouldBeNil)
		})
	})
}

func TestZyReceive(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	Convey("it should call a receive function that returns a hash", t, func() {
		z, _ := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn receive [from msg] (hash %foo (hget msg %bar)))`})
		response, err := z.Receive("fakehash", `{"bar":"baz"}`)
		So(err, ShouldBeNil)
		So(response, ShouldEqual, `{"foo":"baz"}`)
	})

	Convey("it should call a receive function that returns a string", t, func() {
		z, _ := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn receive [from msg] (concat "fish:" (hget msg %bar)))`})
		response, err := z.Receive("fakehash", `{"bar":"baz"}`)
		So(err, ShouldBeNil)
		So(response, ShouldEqual, `"fish:baz"`)
	})

	Convey("it should call a receive function that returns an int", t, func() {
		z, _ := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn receive [from msg] (len (hget msg %bar)))`})
		response, err := z.Receive("fakehash", `{"bar":"baz"}`)
		So(err, ShouldBeNil)
		So(response, ShouldEqual, `3`)
	})
}

func TestZybuildValidate(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

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
	a, _ := NewAgent(LibP2P, "Joe", MakeTestSeed(""))
	h := NewHolochain(a, "some/path", "yaml", Zome{RibosomeType: ZygoRibosomeType})
	h.Config.Loggers.App.New(nil)
	hdr := mkTestHeader("evenNumbers")

	Convey("it should be passing in the correct values", t, func() {
		v, err := NewZygoRibosome(&h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn validateCommit [name entry header pkg sources] (debug name) (debug entry) (debug header) (debug sources) (debug pkg) true)`})
		So(err, ShouldBeNil)
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatString}
		ShouldLog(&h.Config.Loggers.App, `evenNumbers
foo
{"EntryLink":"QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuU2","Type":"evenNumbers","Time":"1970-01-01T00:00:01Z"}
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
		v, err := NewZygoRibosome(&h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn validateCommit [name entry header pkg sources] (cond (== entry "fish") true false))`})
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
		v, err := NewZygoRibosome(&h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn validateCommit [name entry header pkg sources] (cond (== entry "fish") true false))`})
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
		v, err := NewZygoRibosome(&h, &Zome{RibosomeType: ZygoRibosomeType, Code: `(defn validateCommit [name entry header pkg sources] (cond (== (hget entry data:) "fish") true false))`})
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
	defer CleanupTestChain(h, d)

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
	defer CleanupTestChain(h, d)

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

	Convey("get should return entry", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(get "%s")`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		r, err := z.lastResult.(*zygo.SexpHash).HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, `"2"`)
	})

	Convey("get should return entry of sys types", t, func() {
		ShouldLog(h.nucleus.alog, `{"result":"{\"Identity\":\"Herbert \\u003ch@bert.com\\u003e\",\"Revocation\":null,\"PublicKey\":\"CAESIHLUfxjdoEfk8byjsBR+FXxYpYrFTviSBf2BbC0boylT\"}"}`, func() {
			_, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(debug (get "%s"))`, h.agentHash.String())})
			So(err, ShouldBeNil)
		})
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

	commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String()))

	Convey("getLinks function should return the Links", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(getLinks "%s" "4stars")`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		sh := z.lastResult.(*zygo.SexpHash)

		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, fmt.Sprintf(`[{"H":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","E":"","EntryType":"","T":"","Source":"%s"}]`, h.nodeIDStr))
	})
	Convey("getLinks function with load option should return the Links and entries", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(getLinks "%s" "4stars" (hash Load:true))`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		sh := z.lastResult.(*zygo.SexpHash)

		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, fmt.Sprintf(`[{"H":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","E":"{\"firstName\":\"Zippy\",\"lastName\":\"Pinhead\"}","EntryType":"profile","T":"","Source":"%s"}]`, h.nodeIDStr))
	})

	Convey("commit with del link should delete link", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(commit "rating" (hash Links:[(hash LinkAction:HC_LinkAction_Del Base:"%s" Link:"%s" Tag:"4stars")]))`, hash.String(), profileHash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)

		_, err = NewHash(z.lastResult.(*zygo.SexpStr).S)
		So(err, ShouldBeNil)

		links, _ := h.dht.getLinks(hash, "4stars", StatusLive)
		So(fmt.Sprintf("%v", links), ShouldEqual, "[]")
		links, _ = h.dht.getLinks(hash, "4stars", StatusDeleted)
		So(fmt.Sprintf("%v", links), ShouldEqual, fmt.Sprintf("[{QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt    %s}]", h.nodeIDStr))
	})

	Convey("getLinks function with StatusMask option should return deleted Links", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: fmt.Sprintf(`(getLinks "%s" "4stars" (hash StatusMask:HC_Status_Deleted))`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)

		sh := z.lastResult.(*zygo.SexpHash)
		r, err := sh.HashGet(z.env, z.env.MakeSymbol("result"))
		So(err, ShouldBeNil)
		So(r.(*zygo.SexpStr).S, ShouldEqual, fmt.Sprintf(`[{"H":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","E":"","EntryType":"","T":"","Source":"%s"}]`, h.nodeIDStr))
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
		So(r.(*zygo.SexpStr).S, ShouldEqual, `"{\"firstName\":\"Zippy\",\"lastName\":\"ThePinhead\"}"`)
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

	Convey("updateAgent function without options should fail", t, func() {
		_, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType,
			Code: fmt.Sprintf(`(updateAgent (hash))`)})
		So(err.Error(), ShouldEqual, "Zygomys exec error: Error calling 'updateAgent': expecting identity and/or revocation option")
	})

	Convey("updateAgent function should commit a new agent entry", t, func() {
		oldPubKey, _ := h.agent.PubKey().Bytes()
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType,
			Code: fmt.Sprintf(`(updateAgent (hash Identity:"new identity"))`)})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		newAgentHash := z.lastResult.(*zygo.SexpStr).S
		So(h.agentTopHash.String(), ShouldEqual, newAgentHash)
		header := h.chain.Top()
		So(header.Type, ShouldEqual, AgentEntryType)
		So(newAgentHash, ShouldEqual, header.EntryLink.String())
		So(h.agent.Identity(), ShouldEqual, "new identity")
		newPubKey, _ := h.agent.PubKey().Bytes()
		So(fmt.Sprintf("%v", newPubKey), ShouldEqual, fmt.Sprintf("%v", oldPubKey))
		entry, _, _ := h.chain.GetEntry(header.EntryLink)
		So(entry.Content().(AgentEntry).Identity, ShouldEqual, "new identity")
		So(fmt.Sprintf("%v", entry.Content().(AgentEntry).PublicKey), ShouldEqual, fmt.Sprintf("%v", oldPubKey))
	})

	Convey("updateAgent function with revoke option should commit a new agent entry and mark key as modified on DHT", t, func() {
		oldPubKey, _ := h.agent.PubKey().Bytes()
		oldPeer := h.nodeID
		oldKey, _ := NewHash(h.nodeIDStr)
		oldAgentHash := h.agentHash

		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType,
			Code: fmt.Sprintf(`(updateAgent (hash Revocation:"some revocation data"))`)})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		newAgentHash := z.lastResult.(*zygo.SexpStr).S

		So(newAgentHash, ShouldEqual, h.agentTopHash.String())
		So(oldAgentHash.String(), ShouldNotEqual, h.agentTopHash.String())

		header := h.chain.Top()
		So(header.Type, ShouldEqual, AgentEntryType)
		So(newAgentHash, ShouldEqual, header.EntryLink.String())
		newPubKey, _ := h.agent.PubKey().Bytes()
		So(fmt.Sprintf("%v", newPubKey), ShouldNotEqual, fmt.Sprintf("%v", oldPubKey))
		entry, _, _ := h.chain.GetEntry(header.EntryLink)
		revocation := &SelfRevocation{}
		revocation.Unmarshal(entry.Content().(AgentEntry).Revocation)

		w, _ := NewSelfRevocationWarrant(revocation)
		payload, _ := w.Property("payload")

		So(string(payload.([]byte)), ShouldEqual, "some revocation data")
		So(fmt.Sprintf("%v", entry.Content().(AgentEntry).PublicKey), ShouldEqual, fmt.Sprintf("%v", newPubKey))

		// the new Key should be available on the DHT
		newKey, _ := NewHash(h.nodeIDStr)
		data, _, _, _, err := h.dht.get(newKey, StatusDefault, GetMaskDefault)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, string(newPubKey))

		// the old key should be marked as Modifed and we should get the new hash as the data
		data, _, _, _, err = h.dht.get(oldKey, StatusDefault, GetMaskDefault)
		So(err, ShouldEqual, ErrHashModified)
		So(string(data), ShouldEqual, h.nodeIDStr)

		// the new key should be a peerID in the node
		peers := h.node.host.Peerstore().Peers()
		var found bool

		for _, p := range peers {
			pStr := peer.IDB58Encode(p)
			if pStr == h.nodeIDStr {

				found = true
				break
			}
		}
		So(found, ShouldBeTrue)

		// the old peerID should now be in the blockedlist
		peerList, err := h.dht.getList(BlockedList)
		So(err, ShouldBeNil)
		So(len(peerList.Records), ShouldEqual, 1)
		So(peerList.Records[0].ID, ShouldEqual, oldPeer)
		So(h.node.IsBlocked(oldPeer), ShouldBeTrue)
	})

	Convey("updateAgent function should update library values", t, func() {
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType,
			Code: fmt.Sprintf(`(let [x (updateAgent (hash Identity:"new id" evocation:"some revocation data"))] (concat App_Key_Hash "." App_Agent_TopHash "." App_Agent_String))`)})
		So(err, ShouldBeNil)
		z := v.(*ZygoRibosome)
		libVals := z.lastResult.(*zygo.SexpStr).S
		s := strings.Split(libVals, ".")

		So(s[0], ShouldEqual, h.nodeIDStr)
		So(s[1], ShouldEqual, h.agentTopHash.String())
		So(s[2], ShouldEqual, "new id")

	})
}

func TestZyProcessArgs(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	v, _ := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
	z := v.(*ZygoRibosome)

	Convey("it should check for wrong number of args", t, func() {
		zyargs := []zygo.Sexp{zygo.SexpNull, zygo.SexpNull}
		args := []Arg{{}}
		err := zyProcessArgs(z, args, zyargs)
		So(err, ShouldEqual, ErrWrongNargs)

		// test with args that are optional: two that are required and one not
		args = []Arg{{}, {}, {Optional: true}}
		zyargs = []zygo.Sexp{zygo.SexpNull}
		err = zyProcessArgs(z, args, zyargs)
		So(err, ShouldEqual, ErrWrongNargs)

		zyargs = []zygo.Sexp{zygo.SexpNull, zygo.SexpNull, zygo.SexpNull, zygo.SexpNull}
		err = zyProcessArgs(z, args, zyargs)
		So(err, ShouldEqual, ErrWrongNargs)
	})
	Convey("it should convert HashArg to Hash", t, func() {
		hashstr := "QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2"
		args := []Arg{{Name: "foo", Type: HashArg}}
		err := zyProcessArgs(z, args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string")
		var val zygo.Sexp = &zygo.SexpStr{S: hashstr}
		err = zyProcessArgs(z, args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(Hash).String(), ShouldEqual, hashstr)
	})
	Convey("it should treat StringArg as string", t, func() {
		args := []Arg{{Name: "foo", Type: StringArg}}
		err := zyProcessArgs(z, args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string")
		var val zygo.Sexp = &zygo.SexpStr{S: "bar"}
		err = zyProcessArgs(z, args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")
	})
	Convey("it should convert IntArg to int64", t, func() {
		args := []Arg{{Name: "foo", Type: IntArg}}
		err := zyProcessArgs(z, args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be int")
		var val zygo.Sexp = &zygo.SexpInt{Val: 314}
		err = zyProcessArgs(z, args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(int64), ShouldEqual, 314)
	})
	Convey("it should convert BoolArg to bool", t, func() {
		args := []Arg{{Name: "foo", Type: BoolArg}}
		err := zyProcessArgs(z, args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be boolean")
		var val zygo.Sexp = &zygo.SexpBool{Val: true}
		err = zyProcessArgs(z, args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(bool), ShouldEqual, true)
	})

	// create a zygo hash for a test args
	hval, _ := zygo.MakeHash(nil, "hash", z.env)
	hval.HashSet(z.env.MakeSymbol("fname"), &zygo.SexpStr{S: "Jane"})
	hval.HashSet(z.env.MakeSymbol("lname"), &zygo.SexpStr{S: "Smith"})
	Convey("EntryArg should only accept strings for string type entries", t, func() {
		args := []Arg{{Name: "entryType", Type: StringArg}, {Name: "foo", Type: EntryArg}}
		var entryType zygo.Sexp = &zygo.SexpStr{S: "review"}

		err := zyProcessArgs(z, args, []zygo.Sexp{entryType, zygo.SexpNull})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be string")

		err = zyProcessArgs(z, args, []zygo.Sexp{entryType, hval})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be string")

		err = zyProcessArgs(z, args, []zygo.Sexp{entryType, &zygo.SexpInt{Val: 3141}})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be string")

		val := &zygo.SexpStr{S: "bar"}
		err = zyProcessArgs(z, args, []zygo.Sexp{entryType, val})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, "bar")
	})

	Convey("EntryArg should only accept hashes for links type entries", t, func() {
		args := []Arg{{Name: "entryType", Type: StringArg}, {Name: "foo", Type: EntryArg}}
		var entryType zygo.Sexp = &zygo.SexpStr{S: "rating"}

		err := zyProcessArgs(z, args, []zygo.Sexp{entryType, zygo.SexpNull})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be hash")

		err = zyProcessArgs(z, args, []zygo.Sexp{entryType, hval})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, `{"fname":"Jane","lname":"Smith"}`)

		err = zyProcessArgs(z, args, []zygo.Sexp{entryType, &zygo.SexpStr{S: "bar"}})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be hash")

		err = zyProcessArgs(z, args, []zygo.Sexp{entryType, &zygo.SexpInt{Val: 3141}})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be hash")

	})

	Convey("EntryArg should convert all values to JSON for JSON type entries", t, func() {
		args := []Arg{{Name: "entryType", Type: StringArg}, {Name: "foo", Type: EntryArg}}
		var entryType zygo.Sexp = &zygo.SexpStr{S: "profile"}

		err := zyProcessArgs(z, args, []zygo.Sexp{entryType, zygo.SexpNull})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, `undefined`)

		err = zyProcessArgs(z, args, []zygo.Sexp{entryType, hval})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, `{"fname":"Jane","lname":"Smith"}`)

		err = zyProcessArgs(z, args, []zygo.Sexp{entryType, &zygo.SexpStr{S: "bar"}})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, `"bar"`)

		err = zyProcessArgs(z, args, []zygo.Sexp{entryType, &zygo.SexpInt{Val: 3141}})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, `3141`)

	})

	// currently ArgsArg and EntryArg are identical, but we expect this to change
	Convey("it should convert ArgsArg from string or hash", t, func() {
		args := []Arg{{Name: "foo", Type: ArgsArg}}
		err := zyProcessArgs(z, args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be string or hash")
		var val zygo.Sexp = &zygo.SexpStr{S: "bar"}
		err = zyProcessArgs(z, args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")

		// create a zygo hash for a test arg

		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
		env := v.(*ZygoRibosome).env
		hval, _ := zygo.MakeHash(nil, "hash", env)
		hval.HashSet(env.MakeSymbol("fname"), &zygo.SexpStr{S: "Jane"})
		hval.HashSet(env.MakeSymbol("lname"), &zygo.SexpStr{S: "Smith"})
		err = zyProcessArgs(z, args, []zygo.Sexp{hval})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, `{"fname":"Jane","lname":"Smith"}`)

	})

	Convey("it should convert MapArg to a map", t, func() {
		args := []Arg{{Name: "foo", Type: MapArg}}
		err := zyProcessArgs(z, args, []zygo.Sexp{zygo.SexpNull})
		So(err.Error(), ShouldEqual, "argument 1 (foo) should be hash")

		// create a zygo hash as a test arg
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
		env := v.(*ZygoRibosome).env
		var hashstr zygo.Sexp = &zygo.SexpStr{S: "fakehashvalue"}
		hval, _ := zygo.MakeHash(nil, "hash", env)
		hval.HashSet(env.MakeSymbol("H"), hashstr)
		hval.HashSet(env.MakeSymbol("I"), &zygo.SexpInt{Val: 314})

		err = zyProcessArgs(z, args, []zygo.Sexp{hval})
		So(err, ShouldBeNil)
		x := args[0].value.(map[string]interface{})
		So(x["H"].(string), ShouldEqual, "fakehashvalue")
		So(x["I"].(float64), ShouldEqual, 314)
	})

	Convey("it should convert ToStrArg any type to a string", t, func() {
		args := []Arg{{Name: "any", Type: ToStrArg}}
		var val zygo.Sexp = &zygo.SexpStr{S: "bar"}
		err := zyProcessArgs(z, args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "bar")
		val = &zygo.SexpInt{Val: 123}
		err = zyProcessArgs(z, args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "123")
		val = &zygo.SexpBool{Val: true}
		err = zyProcessArgs(z, args, []zygo.Sexp{val})
		So(err, ShouldBeNil)
		So(args[0].value.(string), ShouldEqual, "true")

		// create a zygo hash as a test arg
		v, err := NewZygoRibosome(h, &Zome{RibosomeType: ZygoRibosomeType, Code: ""})
		env := v.(*ZygoRibosome).env
		var hashstr zygo.Sexp = &zygo.SexpStr{S: "fakehashvalue"}
		hval, _ := zygo.MakeHash(nil, "hash", env)
		hval.HashSet(env.MakeSymbol("H"), hashstr)
		hval.HashSet(env.MakeSymbol("I"), &zygo.SexpInt{Val: 314})

		err = zyProcessArgs(z, args, []zygo.Sexp{hval})
		So(args[0].value.(string), ShouldEqual, `{"H":"fakehashvalue","I":314}`)
	})
}
