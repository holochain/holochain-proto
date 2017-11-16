package holochain

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/google/uuid"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	os.Setenv("_HCTEST", "1")
	InitializeHolochain()
	os.Exit(m.Run())
}

func TestNewHolochain(t *testing.T) {
	a, _ := NewAgent(LibP2P, "Joe", MakeTestSeed(""))

	Convey("New should fill Holochain struct with provided values and new UUID", t, func() {

		h := NewHolochain(a, "some/path", "json")
		nUUID := string(uuid.NodeID())
		So(nUUID, ShouldEqual, string(h.nucleus.dna.UUID.NodeID())) // this nodeID is from UUID code, i.e the machine's host (not the LibP2P nodeID below)
		So(h.agent.Identity(), ShouldEqual, "Joe")
		So(h.agent.PrivKey(), ShouldEqual, a.PrivKey())
		So(h.encodingFormat, ShouldEqual, "json")
		So(h.rootPath, ShouldEqual, "some/path")
		So(h.UIPath(), ShouldEqual, "some/path/ui")
		So(h.DNAPath(), ShouldEqual, "some/path/dna")
		So(h.DBPath(), ShouldEqual, "some/path/db")
		nodeID, nodeIDStr, _ := h.agent.NodeID()
		So(h.nodeID, ShouldEqual, nodeID)
		So(h.nodeIDStr, ShouldEqual, nodeIDStr)
		So(h.nodeIDStr, ShouldEqual, peer.IDB58Encode(h.nodeID))

		So(h.nucleus.dna.Progenitor.Identity, ShouldEqual, "Joe")
		pk, _ := a.PubKey().Bytes()
		So(string(h.nucleus.dna.Progenitor.PubKey), ShouldEqual, string(pk))
	})
	Convey("New with Zome should fill them", t, func() {
		z := Zome{Name: "zySampleZome",
			Description: "zome desc",
			Code:        "zome_zySampleZome.zy",
			Entries: []EntryDef{
				{Name: "entryTypeFoo", DataFormat: DataFormatString},
				{Name: "entryTypeBar", DataFormat: DataFormatRawZygo},
			},
		}

		h := NewHolochain(a, "some/path", "yaml", z)
		nz, _ := h.GetZome("zySampleZome")
		So(nz.Description, ShouldEqual, "zome desc")
		So(nz.Code, ShouldEqual, "zome_zySampleZome.zy")
		So(fmt.Sprintf("%v", nz.Entries[0]), ShouldEqual, "{entryTypeFoo string   <nil>}")
		So(fmt.Sprintf("%v", nz.Entries[1]), ShouldEqual, "{entryTypeBar zygo   <nil>}")
	})

}

func TestSetupConfig(t *testing.T) {
	config := Config{}
	Convey("it should set the intervals", t, func() {
		config.Setup()
		So(config.gossipInterval, ShouldEqual, DefaultGossipInterval)
		So(config.bootstrapRefreshInterval, ShouldEqual, BootstrapTTL)
		So(config.routingRefreshInterval, ShouldEqual, DefaultRoutingRefreshInterval)
		So(config.retryInterval, ShouldEqual, DefaultRetryInterval)
	})
}

func TestSetupLogging(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)
	Convey("it should initialize the loggers", t, func() {
		err := h.Config.SetupLogging()
		So(err, ShouldBeNil)
		// test some default configurations
		So(h.Config.Loggers.App.Enabled, ShouldBeFalse)
		So(h.Config.Loggers.DHT.Enabled, ShouldBeFalse)
		So(h.Config.Loggers.Gossip.Enabled, ShouldBeFalse)
		So(h.Config.Loggers.TestFailed.w, ShouldEqual, os.Stderr)
		// test that a sample color got initialized
		So(fmt.Sprintf("%v", h.Config.Loggers.App.color), ShouldEqual, "&{[36] <nil>}")
	})
	Convey("it should initialize the loggers with env vars", t, func() {
		os.Setenv("HCLOG_APP_ENABLE", "0")
		os.Setenv("HCLOG_DHT_ENABLE", "1")
		os.Setenv("HCLOG_GOSSIP_ENABLE", "true")
		os.Setenv("HCLOG_PREFIX", "a prefix:")
		h.Config.Loggers.DHT.Format = "%{message}"
		err := h.Config.SetupLogging()
		So(err, ShouldBeNil)
		So(h.Config.Loggers.App.Enabled, ShouldBeFalse)
		So(h.Config.Loggers.DHT.Enabled, ShouldBeTrue)
		So(h.Config.Loggers.Gossip.Enabled, ShouldBeTrue)
		var buf bytes.Buffer
		h.Config.Loggers.DHT.w = &buf
		h.Config.Loggers.DHT.Log("test")
		So(string(buf.Bytes()), ShouldEqual, "a prefix:test\n")

		// restore env
		os.Unsetenv("HCLOG_APP_ENABLE")
		os.Unsetenv("HCLOG_DHT_ENABLE")
		os.Unsetenv("HCLOG_GOSSIP_ENABLE")
		os.Unsetenv("HCLOG_PREFIX")
		debugLog.SetPrefix("")
		infoLog.SetPrefix("")
	})
}

func TestDebuggingSetup(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("it should look in the environment to know if we should turn on debugging", t, func() {
		val, yes := DebuggingRequestedViaEnv()
		So(yes, ShouldBeFalse)
		os.Setenv("HCDEBUG", "0")
		val, yes = DebuggingRequestedViaEnv()
		So(yes, ShouldBeTrue)
		So(val, ShouldBeFalse)
		os.Setenv("HCDEBUG", "false")
		val, yes = DebuggingRequestedViaEnv()
		So(yes, ShouldBeTrue)
		So(val, ShouldBeFalse)
		os.Setenv("HCDEBUG", "FALSE")
		val, yes = DebuggingRequestedViaEnv()
		So(yes, ShouldBeTrue)
		So(val, ShouldBeFalse)
		os.Setenv("HCDEBUG", "1")
		val, yes = DebuggingRequestedViaEnv()
		So(yes, ShouldBeTrue)
		So(val, ShouldBeTrue)
		os.Setenv("HCDEBUG", "True")
		val, yes = DebuggingRequestedViaEnv()
		So(yes, ShouldBeTrue)
		So(val, ShouldBeTrue)
		os.Setenv("HCDEBUG", "true")
		val, yes = DebuggingRequestedViaEnv()
		So(yes, ShouldBeTrue)
		So(val, ShouldBeTrue)
	})
	Convey("it should setup debugging output", t, func() {
		// test the output of the debug log
		var buf bytes.Buffer
		log := &h.Config.Loggers.Debug
		log.w = &buf
		var enabled = log.Enabled
		log.Enabled = true

		h.Debug("test")
		So(string(buf.Bytes()), ShouldEqual, "HC: holochain_test.go.158: test\n")

		// restore state of debug log
		log.w = os.Stdout
		log.Enabled = enabled
	})
}

func TestPrepare(t *testing.T) {
	Convey("it should fail if the requires version is incorrect", t, func() {
		dna := DNA{DHTConfig: DHTConfig{HashType: "sha1"}, RequiresVersion: Version + 1}
		h := Holochain{}
		h.nucleus = NewNucleus(&h, &dna)
		nextVersion := fmt.Sprintf("%d", Version+1)
		err := h.Prepare()
		So(err.Error(), ShouldEqual, "Chain requires Holochain version "+nextVersion)

	})
	Convey("it should return no err if the requires version is correct", t, func() {
		d, _, h := SetupTestChain("test")
		defer CleanupTestChain(h, d)

		dna := DNA{DHTConfig: DHTConfig{HashType: "sha1"}, RequiresVersion: Version}
		h.nucleus = NewNucleus(h, &dna)
		err := h.Prepare()
		So(err, ShouldBeNil)
	})
	//@todo build out test for other tests for prepare
}

func TestPrepareHashType(t *testing.T) {

	Convey("A bad hash type should return an error", t, func() {
		dna := DNA{DHTConfig: DHTConfig{HashType: "bogus"}}
		h := Holochain{}
		h.nucleus = NewNucleus(&h, &dna)
		err := h.PrepareHashType()
		So(err.Error(), ShouldEqual, "Unknown hash type: bogus")
	})
	Convey("It should initialized fixed and variable sized hashes", t, func() {
		dna := DNA{DHTConfig: DHTConfig{HashType: "sha1"}}
		h := Holochain{}
		h.nucleus = NewNucleus(&h, &dna)
		err := h.PrepareHashType()
		So(err, ShouldBeNil)
		var hash Hash
		err = hash.Sum(h.hashSpec, []byte("test data"))
		So(err, ShouldBeNil)
		So(hash.String(), ShouldEqual, "5duC28CW416wX42vses7TeTeRYwku9")

		h.nucleus.dna.DHTConfig.HashType = "blake2b-256"
		err = h.PrepareHashType()
		So(err, ShouldBeNil)
		err = hash.Sum(h.hashSpec, []byte("test data"))
		So(err, ShouldBeNil)
		So(hash.String(), ShouldEqual, "2DrjgbL49zKmX4P7UgdopSCC7MhfVUySNbRHBQzdDuXgaJSNEg")
	})
}

func TestNewEntry(t *testing.T) {
	d, s := setupTestService()
	defer CleanupTestDir(d)
	n := "test"
	path := filepath.Join(s.Path, n)
	h, err := s.MakeTestingApp(path, "toml", InitializeDB, CloneWithNewUUID, nil)
	if err != nil {
		panic(err)
	}

	entryTypeFoo := `(message (from "art") (to "eric") (contents "test"))`

	now := time.Unix(1, 1) // pick a constant time so the test will always work

	e := GobEntry{C: entryTypeFoo}
	headerHash, header, err := h.NewEntry(now, "entryTypeFoo", &e)
	Convey("parameters passed in should be in the header", t, func() {
		So(err, ShouldBeNil)
		So(header.Time == now, ShouldBeTrue)
		So(header.Type, ShouldEqual, "entryTypeFoo")
		So(header.HeaderLink.IsNullHash(), ShouldBeTrue)
	})
	Convey("the entry hash is correct", t, func() {
		So(err, ShouldBeNil)
		So(header.EntryLink.String(), ShouldEqual, "QmdRXz53TVT9qBYfbXctHyy2GpTNa6YrpAy6ZcDGG8Xhc5")
	})

	// can't check against a fixed hash because signature created each time test runs is
	// different (though valid) so the header will hash to a different value
	Convey("the returned header hash is the SHA256 of the byte encoded header", t, func() {
		b, _ := header.Marshal()
		var hh Hash
		err = hh.Sum(h.hashSpec, b)
		So(err, ShouldBeNil)
		So(headerHash.String(), ShouldEqual, hh.String())
	})

	Convey("it should have signed the entry with my key", t, func() {
		sig := header.Sig
		hash := header.EntryLink.H
		valid, err := h.agent.PrivKey().GetPublic().Verify(hash, sig.S)
		So(err, ShouldBeNil)
		So(valid, ShouldBeTrue)
	})

	Convey("it should store the header and entry to the data store", t, func() {
		s1 := fmt.Sprintf("%v", *header)
		d1 := fmt.Sprintf("%v", entryTypeFoo)

		h2, err := h.chain.Get(headerHash)
		So(err, ShouldBeNil)
		s2 := fmt.Sprintf("%v", *h2)
		So(s2, ShouldEqual, s1)

		Convey("and the returned header should hash to the same value", func() {
			b, _ := (h2).Marshal()
			var hh Hash
			err = hh.Sum(h.hashSpec, b)
			So(err, ShouldBeNil)
			So(headerHash.String(), ShouldEqual, hh.String())
		})

		var d2 interface{}
		var d2t string
		d2, d2t, err = h.chain.GetEntry(h2.EntryLink)
		So(err, ShouldBeNil)
		So(d2t, ShouldEqual, "entryTypeFoo")

		So(d2, ShouldNotBeNil)
		So(d2.(Entry).Content(), ShouldEqual, d1)
	})

	Convey("Top should still work", t, func() {
		hash, err := h.Top()
		So(err, ShouldBeNil)
		So(hash.Equal(&headerHash), ShouldBeTrue)
	})

	e = GobEntry{C: "more data"}
	_, header2, err := h.NewEntry(now, "entryTypeFoo", &e)

	Convey("a second entry should have prev link correctly set", t, func() {
		So(err, ShouldBeNil)
		So(header2.HeaderLink.String(), ShouldEqual, headerHash.String())
	})
}

func TestHeader(t *testing.T) {
	var h1, h2 Header
	h1 = mkTestHeader("entryTypeFoo")

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&h1)
	Convey("it should encode", t, func() {
		So(err, ShouldBeNil)
	})

	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&h2)

	Convey("it should decode", t, func() {
		s1 := fmt.Sprintf("%v", h1)
		s2 := fmt.Sprintf("%v", h2)
		So(err, ShouldBeNil)
		So(s1, ShouldEqual, s2)
	})
}

func TestAddAgentEntry(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("it should add an agent entry to the chain", t, func() {
		headerHash, agentHash, err := h.AddAgentEntry(&FakeRevocation{data: "some revocation data"})
		So(err, ShouldBeNil)

		hdr, err := h.chain.Get(headerHash)
		So(err, ShouldBeNil)

		So(hdr.EntryLink.String(), ShouldEqual, agentHash.String())

		entry, _, err := h.chain.GetEntry(agentHash)
		So(err, ShouldBeNil)

		var a = entry.Content().(AgentEntry)
		So(a.Identity, ShouldEqual, h.agent.Identity())
		pk, _ := h.agent.PubKey().Bytes()
		So(string(a.PublicKey), ShouldEqual, string(pk))
		So(string(a.Revocation), ShouldEqual, "some revocation data")
	})
}

func TestGenChain(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)
	var err error

	Convey("before GenChain call DNAHash call should fail", t, func() {
		h := h.DNAHash()
		So(h.String(), ShouldEqual, "")
	})

	var headerHash Hash
	Convey("GenChain call works", t, func() {
		headerHash, err = h.GenChain()
		So(err, ShouldBeNil)
	})

	var header Header
	Convey("top link should be Key entry", t, func() {
		hdr, err := h.chain.Get(headerHash)
		So(err, ShouldBeNil)
		entry, _, err := h.chain.GetEntry(hdr.EntryLink)
		So(err, ShouldBeNil)
		header = *hdr
		var a = entry.Content().(AgentEntry)
		So(a.Identity, ShouldEqual, h.agent.Identity())
		pk, _ := h.agent.PubKey().Bytes()
		So(string(a.PublicKey), ShouldEqual, string(pk))
		So(string(a.Revocation), ShouldEqual, "")
	})

	var dnaHash Hash
	Convey("next link should be the dna entry", t, func() {
		hdr, err := h.chain.Get(header.HeaderLink)
		So(err, ShouldBeNil)
		entry, et, err := h.chain.GetEntry(hdr.EntryLink)
		So(err, ShouldBeNil)
		So(et, ShouldEqual, DNAEntryType)

		var buf bytes.Buffer
		err = h.EncodeDNA(&buf)
		So(err, ShouldBeNil)
		So(string(entry.Content().([]byte)), ShouldEqual, buf.String())
		dnaHash = hdr.EntryLink
	})

	Convey("holochain id and top should have now been set", t, func() {
		id := h.DNAHash()
		So(err, ShouldBeNil)
		So(id.String(), ShouldEqual, dnaHash.String())
		top, err := h.Top()
		So(err, ShouldBeNil)
		So(top.String(), ShouldEqual, headerHash.String())
	})
}

func TestWalk(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	// add an extra link onto the chain
	entryTypeFoo := `(message (from "art") (to "eric") (contents "test"))`
	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: entryTypeFoo}
	_, _, err := h.NewEntry(now, "entryTypeFoo", &e)
	if err != nil {
		panic(err)
	}

	Convey("walk should call a function on all the elements of a chain", t, func() {

		c := make(map[int]string, 0)
		//	c := make([]string,0)
		idx := 0
		err := h.Walk(func(key *Hash, header *Header, entry Entry) (err error) {
			c[idx] = header.EntryLink.String()
			idx++
			//	c = append(c, header.HeaderLink.String())
			return nil
		}, false)
		So(err, ShouldBeNil)
		id := h.DNAHash()
		So(c[2], ShouldEqual, id.String())
		//	So(c,ShouldEqual,"fish")
	})
}

func TestGetZome(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("it should fail if the zome isn't defined in the DNA", t, func() {
		_, err := h.GetZome("bogusZome")
		So(err.Error(), ShouldEqual, "unknown zome: bogusZome")
	})
	Convey("it should return the Zome structure of a defined zome", t, func() {
		z, err := h.GetZome("zySampleZome")
		So(err, ShouldBeNil)
		So(z.Name, ShouldEqual, "zySampleZome")
	})
}

func TestMakeRibosome(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("it should fail if the zome isn't defined in the DNA", t, func() {
		_, _, err := h.MakeRibosome("bogusZome")
		So(err.Error(), ShouldEqual, "unknown zome: bogusZome")
	})
	Convey("it should make a ribosome based on the type and return the zome def", t, func() {
		v, zome, err := h.MakeRibosome("zySampleZome")
		So(err, ShouldBeNil)
		So(zome.Name, ShouldEqual, "zySampleZome")
		z := v.(*ZygoRibosome)
		_, err = z.env.Run()
		So(err, ShouldBeNil)
	})
}

func TestCall(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("it should call the exposed function", t, func() {
		result, err := h.Call("zySampleZome", "testStrFn1", "arg1 arg2", ZOME_EXPOSURE)
		So(err, ShouldBeNil)
		So(result.(string), ShouldEqual, "result: arg1 arg2")

		result, err = h.Call("zySampleZome", "addEven", "42", ZOME_EXPOSURE)
		So(err, ShouldBeNil)

		ph := h.chain.Top().EntryLink
		So(result.(string), ShouldEqual, ph.String())

		_, err = h.Call("zySampleZome", "addEven", "41", ZOME_EXPOSURE)
		So(err.Error(), ShouldEqual, "Error calling 'commit': Validation Failed")
	})
	Convey("it should fail calls to functions not exposed to the given context", t, func() {
		_, err := h.Call("zySampleZome", "testStrFn1", "arg1 arg2", PUBLIC_EXPOSURE)
		So(err.Error(), ShouldEqual, "function not available")
	})
}

func TestCommit(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	// add an entry onto the chain
	hash := commit(h, "oddNumbers", "7")

	Convey("publicly shared entries should generate a put", t, func() {
		err := h.dht.exists(hash, StatusLive)
		So(err, ShouldBeNil)
	})

	profileHash := commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)

	Convey("it should attach links after commit of Links entry", t, func() {
		commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String()))

		results, err := h.dht.getLinks(hash, "4stars", StatusLive)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", results), ShouldEqual, fmt.Sprintf("[{QmYeinX5vhuA91D3v24YbgyLofw9QAxY6PoATrBHnRwbtt    %s}]", h.nodeIDStr))
	})
}

func TestQuery(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	commit(h, "profile", `{"firstName":"Pebbles","lastName":"Flintstone"}`)
	hash1 := commit(h, "oddNumbers", "7")
	commit(h, "secret", "foo")
	hash2 := commit(h, "oddNumbers", "9")
	commit(h, "secret", "bar")
	commit(h, "secret", "baz")
	commit(h, "profile", `{"firstName":"Zippy","lastName":"Pinhead"}`)
	commit(h, "profile", `{"firstName":"Zerbina","lastName":"Pinhead"}`)

	Convey("query with no options should return entire chain entries only", t, func() {
		results, err := h.Query(nil)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 10)
		So(results[0].Header.Type, ShouldEqual, DNAEntryType)
		So(results[1].Header.Type, ShouldEqual, AgentEntryType)
		So(results[2].Entry.Content(), ShouldEqual, `{"firstName":"Pebbles","lastName":"Flintstone"}`)
		So(results[3].Entry.Content(), ShouldEqual, "7")
		So(results[4].Entry.Content(), ShouldEqual, "foo")
		So(results[5].Entry.Content(), ShouldEqual, "9")
		So(results[6].Entry.Content(), ShouldEqual, "bar")
		So(results[7].Entry.Content(), ShouldEqual, "baz")
		So(results[8].Entry.Content(), ShouldEqual, `{"firstName":"Zippy","lastName":"Pinhead"}`)
		So(results[9].Entry.Content(), ShouldEqual, `{"firstName":"Zerbina","lastName":"Pinhead"}`)
	})
	Convey("query with order should reverse the order", t, func() {
		results, err := h.Query(&QueryOptions{Order: QueryOrder{Ascending: true}})
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 10)
		So(results[9].Header.Type, ShouldEqual, DNAEntryType)
		So(results[8].Header.Type, ShouldEqual, AgentEntryType)
		So(results[7].Entry.Content(), ShouldEqual, `{"firstName":"Pebbles","lastName":"Flintstone"}`)
		So(results[6].Entry.Content(), ShouldEqual, "7")
		So(results[5].Entry.Content(), ShouldEqual, "foo")
		So(results[4].Entry.Content(), ShouldEqual, "9")
		So(results[3].Entry.Content(), ShouldEqual, "bar")
		So(results[2].Entry.Content(), ShouldEqual, "baz")
		So(results[1].Entry.Content(), ShouldEqual, `{"firstName":"Zippy","lastName":"Pinhead"}`)
		So(results[0].Entry.Content(), ShouldEqual, `{"firstName":"Zerbina","lastName":"Pinhead"}`)
	})

	Convey("query with with count and page should select items", t, func() {
		q := &QueryOptions{}
		q.Constrain.Count = 2
		q.Constrain.Page = 2 // zero based
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 2)
		So(results[0].Entry.Content(), ShouldEqual, "foo")
		So(results[1].Entry.Content(), ShouldEqual, "9")
	})

	Convey("query with with count and page partially past end should select items", t, func() {
		q := &QueryOptions{}
		q.Constrain.Count = 4
		q.Constrain.Page = 2 // zero based
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 2)
		So(results[0].Entry.Content(), ShouldEqual, `{"firstName":"Zippy","lastName":"Pinhead"}`)
		So(results[1].Entry.Content(), ShouldEqual, `{"firstName":"Zerbina","lastName":"Pinhead"}`)
	})

	Convey("query with with count and page past end should be empty", t, func() {
		q := &QueryOptions{}
		q.Constrain.Count = 10
		q.Constrain.Page = 1 // zero based
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 0)
	})

	Convey("query with entry type options should return that type only", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"oddNumbers"}
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 2)
		So(results[0].Entry.Content(), ShouldEqual, "7")
		So(results[1].Entry.Content(), ShouldEqual, "9")
	})
	Convey("query with multiple entry type options should return those types only", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"oddNumbers", "secret"}
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 5)
		So(results[0].Entry.Content(), ShouldEqual, "7")
		So(results[1].Entry.Content(), ShouldEqual, "foo")
		So(results[2].Entry.Content(), ShouldEqual, "9")
		So(results[3].Entry.Content(), ShouldEqual, "bar")
		So(results[4].Entry.Content(), ShouldEqual, "baz")
	})
	Convey("query with hash options should return only hashes", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"oddNumbers"}
		q.Return.Hashes = true
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(results[0].Header.EntryLink.String(), ShouldEqual, hash1.String())
		So(results[1].Header.EntryLink.String(), ShouldEqual, hash2.String())
		So(results[0].Entry, ShouldBeNil)
		So(results[1].Entry, ShouldBeNil)
	})
	Convey("query with equals constraint", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"secret"}
		q.Constrain.Equals = "foo"
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 1)
		So(results[0].Entry.Content(), ShouldEqual, "foo")
	})
	Convey("query with contains constraint", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"secret"}
		q.Constrain.Contains = "o"
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 1)
		So(results[0].Entry.Content(), ShouldEqual, "foo")
	})
	Convey("query with matches constraint", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"secret"}
		q.Constrain.Matches = ".a."
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 2)
		So(results[0].Entry.Content(), ShouldEqual, "bar")
		So(results[1].Entry.Content(), ShouldEqual, "baz")
	})
	Convey("query with equals field constraint", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"profile"}
		q.Constrain.Equals = `{"firstName":"Zippy"}`
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 1)
		So(results[0].Entry.Content(), ShouldEqual, `{"firstName":"Zippy","lastName":"Pinhead"}`)
	})
	Convey("query with equals multiple field constraint", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"profile"}
		q.Constrain.Equals = `{"firstName":"Zippy","lastName":"Flintstone"}`
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 2)
		So(results[0].Entry.Content(), ShouldEqual, `{"firstName":"Pebbles","lastName":"Flintstone"}`)
		So(results[1].Entry.Content(), ShouldEqual, `{"firstName":"Zippy","lastName":"Pinhead"}`)

	})
	Convey("query with contains field constraint", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"profile"}
		q.Constrain.Contains = `{"firstName":"Z"}`
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 2)
		So(results[0].Entry.Content(), ShouldEqual, `{"firstName":"Zippy","lastName":"Pinhead"}`)
		So(results[1].Entry.Content(), ShouldEqual, `{"firstName":"Zerbina","lastName":"Pinhead"}`)
	})
	Convey("query with matches field constraint", t, func() {
		q := &QueryOptions{}
		q.Constrain.EntryTypes = []string{"profile"}
		q.Constrain.Matches = `{"firstName":".*b"}`
		results, err := h.Query(q)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 2)
		So(results[0].Entry.Content(), ShouldEqual, `{"firstName":"Pebbles","lastName":"Flintstone"}`)
		So(results[1].Entry.Content(), ShouldEqual, `{"firstName":"Zerbina","lastName":"Pinhead"}`)
	})
}

func TestGetEntryDef(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestDir(d)
	Convey("it should fail on bad entry types", t, func() {
		_, _, err := h.GetEntryDef("foobar")
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "no definition for entry type: foobar")
	})
	Convey("it should get entry definitions", t, func() {
		zome, def, err := h.GetEntryDef("evenNumbers")
		So(err, ShouldBeNil)
		So(zome.Name, ShouldEqual, "zySampleZome")
		So(fmt.Sprintf("%v", def), ShouldEqual, "&{evenNumbers zygo public  <nil>}")
	})
	Convey("it should get sys entry definitions", t, func() {
		zome, def, err := h.GetEntryDef(DNAEntryType)
		So(err, ShouldBeNil)
		So(zome, ShouldBeNil)
		So(def, ShouldEqual, DNAEntryDef)
		zome, def, err = h.GetEntryDef(AgentEntryType)
		So(err, ShouldBeNil)
		So(zome, ShouldBeNil)
		So(def, ShouldEqual, AgentEntryDef)
		zome, def, err = h.GetEntryDef(KeyEntryType)
		So(err, ShouldBeNil)
		So(zome, ShouldBeNil)
		So(def, ShouldEqual, KeyEntryDef)
	})
	Convey("it should get private entry definition", t, func() {
		zome, def, err := h.GetEntryDef("privateData")
		So(err, ShouldBeNil)
		So(zome, ShouldNotBeNil)
		So(def, ShouldNotBeNil)
	})
}

func TestGetPrivateEntryDefs(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestDir(d)
	Convey("it should contain only private entry definition", t, func() {
		_, privateData, _ := h.GetEntryDef("privateData")
		privateDefs := h.GetPrivateEntryDefs()
		So(len(privateDefs), ShouldEqual, 1)
		So(privateDefs[0].Name, ShouldEqual, privateData.Name)
	})
}

//func TestDNADefaults(t *testing.T) {
//	h, err := DecodeDNA(strings.NewReader( [[Zomes]]`
//Name = "test"
//Description = "test-zome"
//RibosomeType = "zygo"`), "toml")
//	if err != nil {
//		return
//	}
//	Convey("it should substitute default values", t, func() {
//		So(h.Zomes[0].Code, ShouldEqual, "test.zy")
//	})
//}

func commit(h *Holochain, entryType, entryStr string) (entryHash Hash) {
	entry := GobEntry{C: entryStr}

	r, err := NewCommitAction(entryType, &entry).Do(h)
	if err != nil {
		panic(err)
	}
	if r != nil {
		entryHash = r.(Hash)
	}
	if err != nil {
		panic(err)
	}
	return
}
