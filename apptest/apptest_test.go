package apptest

import (
	"fmt"
	. "github.com/HC-Interns/holochain-proto"
	. "github.com/HC-Interns/holochain-proto/hash"
	"github.com/HC-Interns/holochain-proto/ui"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// disable UPNP for tests
	os.Setenv("HOLOCHAINCONFIG_ENABLENATUPNP", "false")
	InitializeHolochain()
	os.Exit(m.Run())
}

func TestTestStringReplacements(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	var lastMatches = [3][]string{{"complete match", "1st submatch", "2nd submatch"}}
	history := &history{lastMatches: lastMatches}
	replacements := replacements{h: h, history: history, repetition: "1"}

	Convey("it should replace %dna%", t, func() {
		input := "%dna%"
		output := testStringReplacements(input, &replacements)
		So(output, ShouldEqual, h.DNAHash().String())
	})

	Convey("it should replace %m%", t, func() {
		input := "%m1.2%"
		output := testStringReplacements(input, &replacements)
		So(output, ShouldEqual, "2nd submatch")
	})

	Convey("it should replace %reps%", t, func() {
		input := "%reps%"
		output := testStringReplacements(input, &replacements)
		So(output, ShouldEqual, "1")
	})
	Convey("it should replace %resultX%", t, func() {
		history.results = append(history.results, "foo")
		input := "%result0%"
		output := testStringReplacements(input, &replacements)
		So(output, ShouldEqual, "foo")
		history.results = append(history.results, "bar")
		input = "%result0%-%result1%-%result0%"
		output = testStringReplacements(input, &replacements)
		So(output, ShouldEqual, "foo-bar-foo")
		input = "%result99%"
		output = testStringReplacements(input, &replacements)
		So(output, ShouldEqual, "<bad-result-index>")
		input = "{\"%result%\":0}"
		output = testStringReplacements(input, &replacements)
		So(output, ShouldEqual, "foo")
	})
	Convey("it should replace values in the pairs list", t, func() {
		input := "%fish%"
		replacements.pairs = map[string]string{"%fish%": "doggy"}
		output := testStringReplacements(input, &replacements)
		So(output, ShouldEqual, "doggy")
	})
}

func TestTest(t *testing.T) {
	d, _, h := SetupTestChain("test")
	CleanupTestDir(filepath.Join(d, ".holochain", "test", "test")) // delete the test data created by gen dev

	_, requested := DebuggingRequestedViaEnv()
	// unless env indicates debugging, don't show the test results as this test of testing runs
	if !requested {
		h.Config.Loggers.TestPassed.Enabled = false
		h.Config.Loggers.TestFailed.Enabled = false
		h.Config.Loggers.TestInfo.Enabled = false
	}
	Convey("it should fail if there's no test data", t, func() {
		err := Test(h, nil, false)
		So(err[0].Error(), ShouldEqual, "open "+h.TestPath()+": no such file or directory")
	})
	CleanupTestDir(d)

	d, _, h = SetupTestChain("test")
	defer CleanupTestChain(h, d)

	_, requested = DebuggingRequestedViaEnv()
	// unless env indicates debugging, don't show the test results as this test of testing runs
	if !requested {
		h.Config.Loggers.TestPassed.Enabled = false
		h.Config.Loggers.TestFailed.Enabled = false
		h.Config.Loggers.TestInfo.Enabled = false
	}
	Convey("it should validate on test data", t, func() {
		err := Test(h, nil, false)
		So(err, ShouldBeNil)
	})
	Convey("it should reset the database state and thus run correctly twice", t, func() {
		err := Test(h, nil, false)
		So(err, ShouldBeNil)
	})

	Convey("it should fail the test on incorrect data", t, func() {
		os.Remove(filepath.Join(d, ".holochain", "test", "test", "test_0.json"))
		err := WriteFile([]byte(`{"Tests":[{"Zome":"zySampleZome","FnName":"addEven","Input":"2","Output":"","Err":"bogus error"}]}`), d, ".holochain", "test", "test", "test_0.json")
		So(err, ShouldBeNil)
		err = Test(h, nil, false)[0]
		So(err, ShouldNotBeNil)
		//So(err.Error(), ShouldEqual, "Test: test_0:0\n  Expected Error: bogus error\n  Got: nil\n")
		So(err.Error(), ShouldEqual, "bogus error")
	})
	Convey("it should fail the tests on code with zygo syntax errors", t, func() {
		h.Nucleus().DNA().Zomes[0].Code += "badcode)("
		errs := Test(h, nil, false)
		So(len(errs), ShouldEqual, 12)
	})
	Convey("it should fail the tests on code with js syntax errors", t, func() {
		h.Nucleus().DNA().Zomes[1].Code += "badcode)("
		errs := Test(h, nil, false)
		So(len(errs), ShouldEqual, 3)
	})
}

func TestTestOne(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)

	_, requested := DebuggingRequestedViaEnv()
	if !requested {
		h.Config.Loggers.TestPassed.Enabled = false
		h.Config.Loggers.TestFailed.Enabled = false
		h.Config.Loggers.TestInfo.Enabled = false
	}
	Convey("it should validate on test data", t, func() {

		ShouldLog(&h.Config.Loggers.TestInfo, func() {
			err := TestOne(h, "testSet1", nil, false)
			So(err, ShouldBeNil)
		}, `========================================
Test: 'testSet1' starting...
========================================
Test 'testSet1.0' t+0ms: { zySampleZome addEven 2 %h% <nil>   0s 0s  false 0 false}
`)
	})
}

func TestTestScenario(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)

	_, requested := DebuggingRequestedViaEnv()
	if !requested {
		h.Config.Loggers.TestPassed.Enabled = false
		h.Config.Loggers.TestFailed.Enabled = false
		h.Config.Loggers.TestInfo.Enabled = false
	}
	Convey("it should run a test scenario", t, func() {
		// the sample scenario is supposed to fail
		ShouldLog(&h.Config.Loggers.TestFailed, func() {
			err, errs := TestScenario(h, "sampleScenario", "speaker", map[string]string{"%server%": "server_foo"}, false, nil)
			So(err, ShouldBeNil)
			So(len(errs), ShouldEqual, 1)
		}, `server_foo`)
	})
}

func TestTestBenchmark(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("it should calculate time passed benchmark data", t, func() {
		benchmark := StartBench(h)
		time.Sleep(time.Millisecond)
		benchmark.End()
		So(benchmark.ElapsedTime, ShouldBeGreaterThanOrEqualTo, time.Millisecond)
		So(benchmark.ChainGrowth, ShouldEqual, 0)
		So(benchmark.DHTGrowth, ShouldEqual, 0)
		So(benchmark.BytesSent, ShouldEqual, 0)
	})

	Convey("it should calculate file size growth benchmark data", t, func() {
		benchmark := StartBench(h)
		commit(h, "oddNumbers", "7")
		benchmark.End()
		So(benchmark.ChainGrowth, ShouldBeGreaterThan, 0)
		So(benchmark.DHTGrowth, ShouldBeGreaterThan, 0)
		So(benchmark.CPU, ShouldBeGreaterThanOrEqualTo, 0)
	})

	Convey("it should calculate data sent growth benchmark data", t, func() {
		benchmark := StartBench(h)
		BytesSentChan <- BytesSent{Bytes: int64(100), MsgType: PUT_REQUEST}
		BytesSentChan <- BytesSent{Bytes: int64(500), MsgType: GOSSIP_REQUEST}
		BytesSentChan <- BytesSent{Bytes: int64(200), MsgType: GET_REQUEST}
		benchmark.End()
		So(benchmark.BytesSent, ShouldEqual, 300)
		So(benchmark.GossipSent, ShouldEqual, 500)
	})
}

func TestTestBenchmarks(t *testing.T) {

	d, _, h := SetupTestChain("test")
	defer CleanupTestChain(h, d)

	_, requested := DebuggingRequestedViaEnv()
	if !requested {
		h.Config.Loggers.TestPassed.Enabled = false
		h.Config.Loggers.TestFailed.Enabled = false
		h.Config.Loggers.TestInfo.Enabled = false
	}

	Convey("test should output benchmark info", t, func() {
		ShouldLog(&h.Config.Loggers.TestInfo, func() {
			err := test(h, "testSet1", nil, true)
			So(err, ShouldBeNil)
		},
			`Benchmark for testSet1:8:`,
			`Elapsed time:`,
			`Chain growth:`,
			`DHT growth:`,
			`Bytes sent:`,
			`Benchmark Summary:`,
			`Total elapsed time:`,
			`Total chain growth:`,
			`Total DHT growth:`,
			`Total CPU use:`,
			`Total Bytes sent:`,
		)
	})
}

func commit(h *Holochain, entryType, entryStr string) (entryHash Hash) {
	entry := GobEntry{C: entryStr}
	a := NewCommitAction(entryType, &entry)
	fn := &APIFnCommit{}
	fn.SetAction(a)
	r, err := fn.Call(h)
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

func TestBuildBridgeToCaller(t *testing.T) {
	dCaller, _, hCaller := PrepareTestChain("caller")
	defer CleanupTestChain(hCaller, dCaller)

	callerPort := "31415"
	calleePort := "12356"
	ws := ui.NewWebServer(hCaller, callerPort)
	ws.Start()
	time.Sleep(time.Second * 1)

	dCallee, _, hCallee := PrepareTestChain("callee")
	defer CleanupTestChain(hCallee, dCallee)

	Convey("you can build a bridge to a running caller", t, func() {
		app := BridgeApp{
			BridgeZome: "jsSampleZome",
			DNA:        hCaller.DNAHash(),
			Port:       callerPort,
			BridgeGenesisCallerData: "caller Data",
			BridgeGenesisCalleeData: "callee Data",
			Side: BridgeCaller,
		}
		ShouldLog(&hCallee.Config.Loggers.App, func() {
			err := hCallee.BuildBridgeToCaller(&app, calleePort)
			So(err, ShouldBeNil)
		}, `testGetBridges:[{"Side":1,"Token":"`, fmt.Sprintf(`bridge genesis to-- other side is:%s bridging data:callee Data`, hCallee.DNAHash().String()))

	})
}

func TestBuildBridgeToCallee(t *testing.T) {
	dCaller, _, hCaller := PrepareTestChain("caller")
	defer CleanupTestChain(hCaller, dCaller)

	dCallee, _, hCallee := PrepareTestChain("callee")
	defer CleanupTestChain(hCallee, dCallee)

	calleePort := "12356"
	ws := ui.NewWebServer(hCallee, calleePort)
	ws.Start()
	time.Sleep(time.Second * 1)

	Convey("you can build a bridge to a running callee", t, func() {
		app := BridgeApp{
			BridgeZome: "jsSampleZome",
			Name:       hCallee.Name(),
			DNA:        hCallee.DNAHash(),
			Port:       calleePort,
			BridgeGenesisCallerData: "caller Data",
			BridgeGenesisCalleeData: "callee Data",
			Side: BridgeCallee,
		}
		ShouldLog(&hCaller.Config.Loggers.App, func() {
			err := hCaller.BuildBridgeToCallee(&app)
			So(err, ShouldBeNil)
		}, fmt.Sprintf(`testGetBridges:[{"CalleeApp":"%s"`, hCaller.DNAHash().String()), `"CalleeName":"test"`, `"Side":0`)
	})
}
