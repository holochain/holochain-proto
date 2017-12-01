package apptest

import (
	. "github.com/metacurrency/holochain"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
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
		err := Test(h, nil)
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
		err := Test(h, nil)
		So(err, ShouldBeNil)
	})
	Convey("it should reset the database state and thus run correctly twice", t, func() {
		err := Test(h, nil)
		So(err, ShouldBeNil)
	})

	Convey("it should fail the test on incorrect data", t, func() {
		os.Remove(filepath.Join(d, ".holochain", "test", "test", "test_0.json"))
		err := WriteFile([]byte(`{"Tests":[{"Zome":"zySampleZome","FnName":"addEven","Input":"2","Output":"","Err":"bogus error"}]}`), d, ".holochain", "test", "test", "test_0.json")
		So(err, ShouldBeNil)
		err = Test(h, nil)[0]
		So(err, ShouldNotBeNil)
		//So(err.Error(), ShouldEqual, "Test: test_0:0\n  Expected Error: bogus error\n  Got: nil\n")
		So(err.Error(), ShouldEqual, "bogus error")
	})
	Convey("it should fail the tests on code with zygo syntax errors", t, func() {
		h.Nucleus().DNA().Zomes[0].Code += "badcode)("
		errs := Test(h, nil)
		So(len(errs), ShouldEqual, 12)
	})
	Convey("it should fail the tests on code with js syntax errors", t, func() {
		h.Nucleus().DNA().Zomes[1].Code += "badcode)("
		errs := Test(h, nil)
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

		ShouldLog(&h.Config.Loggers.TestInfo, `========================================
Test: 'testSet1' starting...
========================================
Test 'testSet1.0' t+0ms: { zySampleZome addEven 2 %h%   0s 0s  false 0}
`, func() {
			err := TestOne(h, "testSet1", nil)
			So(err, ShouldBeNil)
		})
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
		ShouldLog(&h.Config.Loggers.TestFailed, `server_foo`, func() {
			err, errs := TestScenario(h, "sampleScenario", "speaker", map[string]string{"%server%": "server_foo"})
			So(err, ShouldBeNil)
			So(len(errs), ShouldEqual, 1)
		})
	})
}
