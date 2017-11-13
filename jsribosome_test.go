package holochain

import (
	"encoding/json"
	"fmt"
	b58 "github.com/jbenet/go-base58"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	"github.com/robertkrimen/otto"
	. "github.com/smartystreets/goconvey/convey"
	"strings"
	"testing"
)

func TestNewJSRibosome(t *testing.T) {

	Convey("new should create a ribosome", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestChain(h, d)
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `1 + 1`})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		i, _ := z.lastResult.ToInteger()
		So(i, ShouldEqual, 2)
	})
	Convey("new fail to create ribosome when code is bad", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestChain(h, d)
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: "\n1+ )"})
		So(v, ShouldBeNil)
		So(err.Error(), ShouldEqual, "Error executing JavaScript: (anonymous): Line 2:4 Unexpected token )")
	})

	Convey("it should have an App structure:", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestChain(h, d)

		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)

		_, err = z.Run("App.Name")
		So(err, ShouldBeNil)
		s, _ := z.lastResult.ToString()
		So(s, ShouldEqual, h.Name())

		_, err = z.Run("App.DNA.Hash")
		So(err, ShouldBeNil)
		s, _ = z.lastResult.ToString()
		So(s, ShouldEqual, h.dnaHash.String())

		_, err = z.Run("App.Agent.Hash")
		So(err, ShouldBeNil)
		s, _ = z.lastResult.ToString()
		So(s, ShouldEqual, h.agentHash.String())

		_, err = z.Run("App.Agent.TopHash")
		So(err, ShouldBeNil)
		s, _ = z.lastResult.ToString()
		So(s, ShouldEqual, h.agentTopHash.String()) // top an agent are the same at startup
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
		defer CleanupTestChain(h, d)

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

		_, err = z.Run("HC.Bridge.From")
		So(err, ShouldBeNil)
		i, _ = z.lastResult.ToInteger()
		So(i, ShouldEqual, BridgeFrom)

		_, err = z.Run("HC.Bridge.To")
		So(err, ShouldBeNil)
		i, _ = z.lastResult.ToInteger()
		So(i, ShouldEqual, BridgeTo)

	})

	Convey("should have the built in functions:", t, func() {
		d, _, h := PrepareTestChain("test")
		defer CleanupTestChain(h, d)

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
			_, err = z.Run(`makeHash("oddNumbers","3")`)
			So(err, ShouldBeNil)
			z := v.(*JSRibosome)
			hash1, err := NewHash(z.lastResult.String())
			So(err, ShouldBeNil)
			So(hash1.String(), ShouldEqual, hash.String())

			_, err = z.Run(`makeHash("profile",{"firstName":"Zippy","lastName":"Pinhead"})`)
			So(err, ShouldBeNil)
			hash1, err = NewHash(z.lastResult.String())
			So(err, ShouldBeNil)
			So(hash1.String(), ShouldEqual, profileHash.String())
		})

		Convey("getBridges", func() {
			_, err = z.Run(`getBridges()`)
			So(err, ShouldBeNil)
			z := v.(*JSRibosome)
			s, _ := z.lastResult.Export()
			So(fmt.Sprintf("%v", s), ShouldEqual, "[]")

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

			ShouldLog(h.nucleus.alog, fmt.Sprintf(`[{"Side":0,"ToApp":"QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqto"},{"Side":1,"Token":"%s"}]`, token), func() {
				_, err := z.Run(`testGetBridges()`)
				So(err, ShouldBeNil)
			})

		})

		// Sign - this methord signs the data that is passed with the user's privKey and returns the signed data
		Convey("sign", func() {
			d, _, h := PrepareTestChain("test")
			defer CleanupTestChain(h, d)

			v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType})
			So(err, ShouldBeNil)
			z := v.(*JSRibosome)
			// sig should match the value that is returned
			privKey := h.agent.PrivKey()
			sig, err := privKey.Sign([]byte("3"))
			//test1
			_, err = z.Run(`sign("3")`)
			So(err, ShouldBeNil)
			//z := v.(*JSRibosome)
			So(z.lastResult.String(), ShouldEqual, string(sig))
			//test2
			sig, err = privKey.Sign([]byte("{\"firstName\":\"jackT\",\"lastName\":\"hammer\"}"))
			_, err = z.Run(`sign('{"firstName":"jackT","lastName":"hammer"}')`)
			So(err, ShouldBeNil)
			So(z.lastResult.String(), ShouldEqual, string(sig))
		})

		//Verifying signature of a perticular user
		// sig will be signed by the user and We will verifySignature i.e verify if the uses we know signed it
		Convey("verifySignature", func() {
			d, _, h := PrepareTestChain("test")
			defer CleanupTestChain(h, d)
			v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType})
			So(err, ShouldBeNil)
			z := v.(*JSRibosome)
			privKey := h.agent.PrivKey()
			pubKey := privKey.GetPublic()
			var pubKeyBytes []byte
			pubKeyBytes, err = ic.MarshalPublicKey(pubKey)
			if err != nil {
				panic(err)
			}
			//verifySignature function SUCESS Condition
			sig, err := privKey.Sign([]byte("3"))
			_, err = z.Run(fmt.Sprintf(`verifySignature("%s","%s","%s")`, b58.Encode((sig)), "3", b58.Encode(pubKeyBytes)))
			So(err, ShouldBeNil)
			So(z.lastResult.String(), ShouldEqual, "true")
			//verifySignature function FAILURE Condition
			_, err = z.Run(fmt.Sprintf(`verifySignature("%s","%s","%s")`, b58.Encode(sig), "34", b58.Encode(pubKeyBytes)))
			So(err, ShouldBeNil)
			So(z.lastResult.String(), ShouldEqual, "false")
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
		Convey("bridge", func() {
			_, err := z.Run(`bridge("QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHw","zySampleZome","testStrFn1","foo")`)
			So(err, ShouldBeNil)
			result := z.lastResult.String()
			So(result, ShouldEqual, "HolochainError: no active bridge")

			// TODO
			// This test can't work because of the import cycle that we can't import
			// apptest into holochain.
			// The solution is to have a different method other than web access, i.e. direct call
			// for the bridge.

			/*
				// set up a bridge app

				d, s, h := PrepareTestChain("test")
				defer CleanupTestChain(h, d)

				h2, err := s.MakeTestingApp(filepath.Join(s.Path, "test2"), "json",nil)
				if err != nil {
					panic(err)
				}
				bridgeApps := []BridgeApp{BridgeApp{
					H:    h2,
					Side: BridgeTo,
					Port: "31111",
				}}
				bridgeAppServers, err := BuildBridges(h, bridgeApps)
				if err != nil {
					panic(err)
				}
				_, err := z.Run(fmt.Sprintf(`bridge("%s","zySampleZome","testStrFn1","foo")`, h2.DNAHash().String()))
				So(err, ShouldBeNil)
				result := z.lastResult.String()
				So(result, ShouldEqual, "result: foo")
				bridgeAppServers[0].Stop()
				bridgeAppServers[0].Wait()
			*/
		})
		Convey("send", func() {
			ShouldLog(h.nucleus.alog, `result was: "{\"pong\":\"foobar\"}"`, func() {
				_, err := z.Run(`debug("result was: "+JSON.stringify(send(App.Key.Hash,{ping:"foobar"})))`)
				So(err, ShouldBeNil)
			})
		})
		Convey("send async", func() {
			ShouldLog(h.nucleus.alog, `async result of message with 123 was: {"pong":"foobar"}`, func() {
				_, err := z.Run(`send(App.Key.Hash,{ping:"foobar"},{Callback:{Function:"asyncPing",ID:"123"}})`)
				So(err, ShouldBeNil)
				err = <-h.asyncSends
				So(err, ShouldBeNil)
			})
		})
		Convey("send async with 100ms timeout", func() {
			_, err := z.Run(`send(App.Key.Hash,{block:true},{Callback:{Function:"asyncPing",ID:"123"},Timeout:100})`)
			So(err, ShouldBeNil)
			err = <-h.asyncSends
			So(err, ShouldBeError, SendTimeoutErr)
		})
	})
}

func TestJSQuery(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	zome, _ := h.GetZome("jsSampleZome")
	v, err := NewJSRibosome(h, zome)
	if err != nil {
		panic(err)
	}
	z := v.(*JSRibosome)

	Convey("query", t, func() {
		// add entries onto the chain to get hash values for testing
		hash := commit(h, "oddNumbers", "3")
		commit(h, "secret", "foo")
		commit(h, "oddNumbers", "7")
		commit(h, "secret", "bar")
		profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)
		commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String()))

		ShouldLog(h.nucleus.alog, `[3,7]`, func() {
			_, err := z.Run(`debug(query({Constrain:{EntryTypes:["oddNumbers"]}}))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `["QmSwMfay3iCynzBFeq9rPzTMTnnuQSMUSe84whjcC9JPAo","QmfMPAEdN1BB9imcz97NsaYYaWEN3baC5aSDXqJSiWt4e6"]`, func() {
			_, err := z.Run(`debug(query({Return:{Hashes:true},Constrain:{EntryTypes:["oddNumbers"]}}))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `[{"Entry":3,"Hash":"QmSwMfay3iCynzBFeq9rPzTMTnnuQSMUSe84whjcC9JPAo"},{"Entry":7,"Hash":"QmfMPAEdN1BB9imcz97NsaYYaWEN3baC5aSDXqJSiWt4e6"}]`, func() {
			_, err := z.Run(`debug(query({Return:{Hashes:true,Entries:true},Constrain:{EntryTypes:["oddNumbers"]}}))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `[{"Entry":3,"Header":{"EntryLink":"QmSwMfay3iCynzBFeq9rPzTMTnnuQSMUSe84whjcC9JPAo","HeaderLink":"Qm`, func() {
			_, err := z.Run(`debug(query({Return:{Headers:true,Entries:true},Constrain:{EntryTypes:["oddNumbers"]}}))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `["foo","bar"]`, func() {
			_, err := z.Run(`debug(query({Constrain:{EntryTypes:["secret"]}}))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `[{"firstName":"Zippy","lastName":"Pinhead"}]`, func() {
			_, err := z.Run(`debug(query({Constrain:{EntryTypes:["profile"]}}))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `[{"Identity":"Herbert \u003ch@bert.com\u003e","PublicKey":"CAESIHLUfxjdoEfk8byjsBR+FXxYpYrFTviSBf2BbC0boylT","Revocation":null}]`, func() {
			_, err := z.Run(`debug(query({Constrain:{EntryTypes:["%agent"]}}))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `{"message":"data format not implemented: _DNA","name":"HolochainError"}`, func() {
			_, err := z.Run(`debug(query({Constrain:{EntryTypes:["%dna"]}}))`)
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `[{"Links":[{"Base":"QmSwMfay3iCynzBFeq9rPzTMTnnuQSMUSe84whjcC9JPAo","Link":"QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt","Tag":"4stars"}]}]`, func() {
			_, err := z.Run(`debug(query({Constrain:{EntryTypes:["rating"]}}))`)
			So(err, ShouldBeNil)
		})
	})
}

func TestJSGenesis(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	Convey("it should fail if the genesis function returns false", t, func() {
		z, _ := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function genesis() {return false}`})
		err := z.ChainGenesis()
		So(err.Error(), ShouldEqual, "genesis failed")
	})
	Convey("it should work if the genesis function returns true", t, func() {
		z, _ := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function genesis() {return true}`})
		err := z.ChainGenesis()
		So(err, ShouldBeNil)
	})
}

func TestJSBridgeGenesis(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	fakeToApp, _ := NewHash("QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx")
	Convey("it should fail if the bridge genesis function returns false", t, func() {

		ShouldLog(&h.Config.Loggers.App, h.dnaHash.String()+" test data", func() {
			z, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function bridgeGenesis(side,app,data) {debug(app+" "+data);if (side==HC.Bridge.From) {return false;} return true;}`})
			So(err, ShouldBeNil)
			err = z.BridgeGenesis(BridgeFrom, h.dnaHash, "test data")
			So(err.Error(), ShouldEqual, "bridgeGenesis failed")
		})
	})
	Convey("it should work if the genesis function returns true", t, func() {
		ShouldLog(&h.Config.Loggers.App, fakeToApp.String()+" test data", func() {
			z, _ := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function bridgeGenesis(side,app,data) {debug(app+" "+data);if (side==HC.Bridge.From) {return false;} return true;}`})
			err := z.BridgeGenesis(BridgeTo, fakeToApp, "test data")
			So(err, ShouldBeNil)
		})
	})
}

func TestJSReceive(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	Convey("it should call a receive function", t, func() {
		z, _ := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function receive(from,msg) {return {foo:msg.bar}}`})
		response, err := z.Receive("fakehash", `{"bar":"baz"}`)
		So(err, ShouldBeNil)
		So(response, ShouldEqual, `{"foo":"baz"}`)
	})
}

func TestJSbuildValidate(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

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
	defer CleanupTestChain(h, d)
	//	a, _ := NewAgent(LibP2P, "Joe", MakeTestSeed(""))
	//	h := NewHolochain(a, "some/path", "yaml", Zome{RibosomeType:JSRibosomeType,})
	//	a := h.agent
	h.Config.Loggers.App.Format = ""
	h.Config.Loggers.App.New(nil)
	hdr := mkTestHeader("evenNumbers")
	pkg, _ := MakePackage(h, PackagingReq{PkgReqChain: int64(PkgReqChainOptFull)})
	vpkg, _ := MakeValidationPackage(h, &pkg)

	Convey("it should be passing in the correct values", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function validateCommit(name,entry,header,pkg,sources) {debug(name);debug(entry);debug(JSON.stringify(header));debug(JSON.stringify(sources));debug(JSON.stringify(pkg));return true};`})
		So(err, ShouldBeNil)
		d := EntryDef{Name: "evenNumbers", DataFormat: DataFormatString}
		ShouldLog(&h.Config.Loggers.App, `evenNumbers
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
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function validateCommit(name,entry,header,pkg,sources) { return (entry=="fish")};`})
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
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function validateCommit(name,entry,header,pkg,sources) { return (entry=="fish")};`})
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
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: `function validateCommit(name,entry,header,pkg,sources) { return (entry.data=="fish")};`})
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
		So(jsSanitizeString(`\"`), ShouldEqual, `\\\"`)
		So(jsSanitizeString("\"x\ny"), ShouldEqual, "\\\"x\\ny")
	})
}

func TestJSExposeCall(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

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
		So(result.(string), ShouldEqual, "result: fish \"\nzippy\"")
	})
	Convey("should fail on bad JSON", t, func() {
		times2, _ := zome.GetFunctionDef("testJsonFn1")
		_, err := z.Call(times2, "{\"input\n\": 2}")
		So(err, ShouldBeError)
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
	defer CleanupTestChain(h, d)

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat6x5HEhc1TVGs11tmfNSzkqh2")
	Convey("get should return hash not found if it doesn't exist", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s");`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		So(z.lastResult.String(), ShouldEqual, "HolochainError: hash not found")
	})

	// add an entry onto the chain
	hash = commit(h, "oddNumbers", "7")

	Convey("get should return entry", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s");`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", x), ShouldEqual, `7`)
	})

	Convey("get should return entry of sys types", t, func() {
		ShouldLog(h.nucleus.alog, `{"Identity":"Herbert \u003ch@bert.com\u003e","PublicKey":[8,1,18,32,114,212,127,24,221,160,71,228,241,188,163,176,20,126,21,124,88,165,138,197,78,248,146,5,253,129,108,45,27,163,41,83],"Revocation":[]}`, func() {
			_, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`debug(get("%s"));`, h.agentHash.String())})
			So(err, ShouldBeNil)
		})
		ShouldLog(h.nucleus.alog, `[8,1,18,32,114,212,127,24,221,160,71,228,241,188,163,176,20,126,21,124,88,165,138,197,78,248,146,5,253,129,108,45,27,163,41,83]`, func() {
			_, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`debug(get("%s"));`, h.nodeID.Pretty())})
			So(err, ShouldBeNil)
		})
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
		So(obj["Entry"], ShouldEqual, `7`)
		So(obj["EntryType"].(string), ShouldEqual, `oddNumbers`)
		So(fmt.Sprintf("%v", obj["Sources"]), ShouldEqual, fmt.Sprintf("[%v]", h.nodeIDStr))
	})

	profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)
	reviewHash := commit(h, "review", "this is my bogus review of some thing")

	commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"},{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String(), hash.String(), reviewHash.String()))

	Convey("getLinks should return the Links", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`getLinks("%s","4stars");`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		So(z.lastResult.Class(), ShouldEqual, "Array")
		links, _ := z.lastResult.Export()
		l0 := links.([]map[string]interface{})[0]
		l1 := links.([]map[string]interface{})[1]

		So(fmt.Sprintf("%v", l0["Hash"]), ShouldEqual, reviewHash.String())
		So(fmt.Sprintf("%v", l1["Hash"]), ShouldEqual, profileHash.String())
	})

	Convey("getLinks with empty tag should return the Links and tags", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`getLinks("%s","");`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		So(z.lastResult.Class(), ShouldEqual, "Array")
		links, _ := z.lastResult.Export()
		l0 := links.([]map[string]interface{})[0]
		l1 := links.([]map[string]interface{})[1]

		So(fmt.Sprintf("%v", l0["Hash"]), ShouldEqual, reviewHash.String())
		So(fmt.Sprintf("%v", l0["Tag"]), ShouldEqual, "4stars")
		So(fmt.Sprintf("%v", l1["Hash"]), ShouldEqual, profileHash.String())
		So(fmt.Sprintf("%v", l1["Tag"]), ShouldEqual, "4stars")

	})

	Convey("getLinks with load option should return the Links and entries", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`getLinks("%s","4stars",{Load:true});`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		So(z.lastResult.Class(), ShouldEqual, "Array")
		links, _ := z.lastResult.Export()
		l0 := links.([]map[string]interface{})[0]
		l1 := links.([]map[string]interface{})[1]
		So(l1["Hash"], ShouldEqual, profileHash.String())
		lp := l1["Entry"].(map[string]interface{})
		So(fmt.Sprintf("%v", lp["firstName"]), ShouldEqual, "Zippy")
		So(fmt.Sprintf("%v", lp["lastName"]), ShouldEqual, "Pinhead")
		So(l1["EntryType"], ShouldEqual, "profile")
		So(l1["Source"], ShouldEqual, h.nodeIDStr)

		So(l0["Hash"], ShouldEqual, reviewHash.String())
		So(fmt.Sprintf("%v", l0["Entry"]), ShouldEqual, `this is my bogus review of some thing`)
		So(l0["EntryType"], ShouldEqual, "review")
	})

	Convey("getLinks with load option should return the Links and entries for linked sys types", t, func() {
		commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"},{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, profileHash.String(), h.nodeIDStr, profileHash.String(), h.agentHash.String()))
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`getLinks("%s","4stars",{Load:true});`, profileHash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		So(z.lastResult.Class(), ShouldEqual, "Array")
		links, _ := z.lastResult.Export()
		l0 := links.([]map[string]interface{})[0]
		l1 := links.([]map[string]interface{})[1]
		So(l1["Hash"], ShouldEqual, h.agentHash.String())
		lp := l1["Entry"].(map[string]interface{})
		So(fmt.Sprintf("%v", lp["Identity"]), ShouldEqual, "Herbert <h@bert.com>")
		So(fmt.Sprintf("%v", lp["PublicKey"]), ShouldEqual, "CAESIHLUfxjdoEfk8byjsBR+FXxYpYrFTviSBf2BbC0boylT")
		So(l1["EntryType"], ShouldEqual, AgentEntryType)
		So(l1["Source"], ShouldEqual, h.nodeIDStr)

		So(l0["Hash"], ShouldEqual, h.nodeIDStr)
		So(fmt.Sprintf("%v", l0["Entry"]), ShouldEqual, "CAESIHLUfxjdoEfk8byjsBR+FXxYpYrFTviSBf2BbC0boylT")
		So(l0["EntryType"], ShouldEqual, KeyEntryType)
	})

	Convey("commit with del link should delete link", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`commit("rating",{Links:[{"LinkAction":HC.LinkAction.Del,Base:"%s",Link:"%s",Tag:"4stars"}]});`, hash.String(), profileHash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		_, err = NewHash(z.lastResult.String())
		So(err, ShouldBeNil)

		links, _ := h.dht.getLinks(hash, "4stars", StatusLive)
		So(fmt.Sprintf("%v", links), ShouldEqual, fmt.Sprintf("[{QmWbbUf6G38hT27kmrQ5UYFbXUPTGokKvDiaQbczFYNjuN    %s}]", h.nodeIDStr))
		links, _ = h.dht.getLinks(hash, "4stars", StatusDeleted)
		So(fmt.Sprintf("%v", links), ShouldEqual, fmt.Sprintf("[{QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt    %s}]", h.nodeIDStr))
	})

	Convey("getLinks with StatusMask option should return deleted Links", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`getLinks("%s","4stars",{StatusMask:HC.Status.Deleted});`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)

		So(z.lastResult.Class(), ShouldEqual, "Array")
		links, _ := z.lastResult.Export()
		l0 := links.([]map[string]interface{})[0]
		So(l0["Hash"], ShouldEqual, profileHash.String())
	})

	Convey("getLinks with quotes in tags should work", t, func() {

		commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"\"quotes!\""}]}`, hash.String(), profileHash.String()))
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`getLinks("%s","\"quotes!\"");`, hash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		So(z.lastResult.Class(), ShouldEqual, "Array")
		links, _ := z.lastResult.Export()
		l0 := links.([]map[string]interface{})[0]

		So(fmt.Sprintf("%v", l0["Hash"]), ShouldEqual, profileHash.String())
	})

	Convey("update should commit a new entry and on DHT mark item modified", t, func() {
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`update("profile",{firstName:"Zippy",lastName:"ThePinhead"},"%s")`, profileHash.String())})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		profileHashStr2 := z.lastResult.String()

		header := h.chain.Top()
		So(profileHashStr2, ShouldEqual, header.EntryLink.String())
		So(header.Change.Action, ShouldEqual, ModAction)
		So(header.Change.Hash.String(), ShouldEqual, profileHash.String())

		// the entry should be marked as Modifed
		data, _, _, _, err := h.dht.get(profileHash, StatusDefault, GetMaskDefault)
		So(err, ShouldEqual, ErrHashModified)
		So(string(data), ShouldEqual, profileHashStr2)

		// but a regular get, should resolve through
		v, err = NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType, Code: fmt.Sprintf(`get("%s");`, profileHash.String())})
		z = v.(*JSRibosome)
		x, err := z.lastResult.Export()
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", x), ShouldEqual, `{"firstName":"Zippy","lastName":"ThePinhead"}`)
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
		So(fmt.Sprintf("%v", x), ShouldEqual, `7`)
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

		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType,
			Code: fmt.Sprintf(`updateAgent({Revocation:"some revocation data"})`)})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		newAgentHash := z.lastResult.String()
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
		v, err := NewJSRibosome(h, &Zome{RibosomeType: JSRibosomeType,
			Code: fmt.Sprintf(`updateAgent({Identity:"new id",Revocation:"some revocation data"});App.Key.Hash+"."+App.Agent.TopHash+"."+App.Agent.String`)})
		So(err, ShouldBeNil)
		z := v.(*JSRibosome)
		libVals := z.lastResult.String()
		s := strings.Split(libVals, ".")

		So(s[0], ShouldEqual, h.nodeIDStr)
		So(s[1], ShouldEqual, h.agentTopHash.String())
		So(s[2], ShouldEqual, "new id")

	})
}

func TestJSProcessArgs(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

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

	Convey("EntryArg should only accept strings for string type entries", t, func() {
		args := []Arg{{Name: "entryType", Type: StringArg}, {Name: "foo", Type: EntryArg}}
		entryType, _ := z.vm.ToValue("review")

		err := jsProcessArgs(z, args, []otto.Value{entryType, nilValue})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be string")

		val, _ := z.vm.ToValue("bar")
		err = jsProcessArgs(z, args, []otto.Value{entryType, val})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, "bar")

		// create an otto.object for a test arg
		val, _ = z.vm.ToValue(map[string]string{"H": "foo", "E": "bar"})
		err = jsProcessArgs(z, args, []otto.Value{entryType, val})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be string")

		val, _ = z.vm.ToValue(3.1415)
		err = jsProcessArgs(z, args, []otto.Value{entryType, val})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be string")
	})

	Convey("EntryArg should only accept objects for links type entries", t, func() {
		args := []Arg{{Name: "entryType", Type: StringArg}, {Name: "foo", Type: EntryArg}}
		entryType, _ := z.vm.ToValue("rating")

		err := jsProcessArgs(z, args, []otto.Value{entryType, nilValue})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be object")

		val, _ := z.vm.ToValue("bar")
		err = jsProcessArgs(z, args, []otto.Value{entryType, val})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be object")

		// create an otto.object for a test arg
		val, _ = z.vm.ToValue(map[string]string{"H": "foo", "E": "bar"})
		err = jsProcessArgs(z, args, []otto.Value{entryType, val})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, `{"E":"bar","H":"foo"}`)

		val, _ = z.vm.ToValue(3.1415)
		err = jsProcessArgs(z, args, []otto.Value{entryType, val})
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "argument 2 (foo) should be object")
	})

	Convey("EntryArg should convert all values to JSON for JSON type entries", t, func() {
		args := []Arg{{Name: "entryType", Type: StringArg}, {Name: "foo", Type: EntryArg}}
		entryType, _ := z.vm.ToValue("profile")

		err := jsProcessArgs(z, args, []otto.Value{entryType, nilValue})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, "undefined")

		val, _ := z.vm.ToValue("bar")
		err = jsProcessArgs(z, args, []otto.Value{entryType, val})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, `"bar"`)

		val, _ = z.vm.ToValue(3.1415)
		err = jsProcessArgs(z, args, []otto.Value{entryType, val})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, `3.1415`)

		// create an otto.object for a test arg
		val, _ = z.vm.ToValue(map[string]string{"H": "foo", "E": "bar"})
		err = jsProcessArgs(z, args, []otto.Value{entryType, val})
		So(err, ShouldBeNil)
		So(args[1].value.(string), ShouldEqual, `{"E":"bar","H":"foo"}`)
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
