package apptest

import (
	"fmt"
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
	CleanupTestDir(d)
	var lastMatches = [3][]string{{"complete match", "1st submatch", "2nd submatch"}}

	Convey("it should replace %dna%", t, func() {
		input := "%dna%"
		output := TestStringReplacements(h, input, "", "", "", &lastMatches)
		So(output, ShouldEqual, h.DNAHash().String())
	})

	Convey("it should replace %m%", t, func() {
		input := "%m1.2%"
		output := TestStringReplacements(h, input, "", "", "", &lastMatches)
		So(output, ShouldEqual, "2nd submatch")
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
	defer CleanupTestDir(d)

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

	Convey("it should fail the test on incorrect input types", t, func() {
		os.Remove(filepath.Join(d, ".holochain", "test", "test", "test_0.json"))
		err := WriteFile([]byte(`[{"Zome":"zySampleZome","FnName":"addEven","Input":2,"Output":"%h%","Err":""}]`), d, ".holochain", "test", "test", "test_0.json")
		So(err, ShouldBeNil)
		err = Test(h, nil)[0]
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Input was not an expected type: float64")
	})
	Convey("it should fail the test on incorrect data", t, func() {
		os.Remove(filepath.Join(d, ".holochain", "test", "test", "test_0.json"))
		err := WriteFile([]byte(`[{"Zome":"zySampleZome","FnName":"addEven","Input":"2","Output":"","Err":"bogus error"}]`), d, ".holochain", "test", "test", "test_0.json")
		So(err, ShouldBeNil)
		err = Test(h, nil)[0]
		So(err, ShouldNotBeNil)
		//So(err.Error(), ShouldEqual, "Test: test_0:0\n  Expected Error: bogus error\n  Got: nil\n")
		So(err.Error(), ShouldEqual, "bogus error")
	})
}

func TestTestOne(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestDir(d)
	if os.Getenv("DEBUG") != "" {
		h.Config.Loggers.TestPassed.Enabled = false
		h.Config.Loggers.TestFailed.Enabled = false
		h.Config.Loggers.TestInfo.Enabled = false
	}
	Convey("it should validate on test data", t, func() {

		ShouldLog(&h.Config.Loggers.TestInfo, `========================================
Test: 'testSet1' starting...
========================================
Test 'testSet1.0' t+0ms: { zySampleZome addEven 2 %h%   0s  false}
`, func() {
			err := TestOne(h, "testSet1", nil)
			So(err, ShouldBeNil)
		})
	})
}

func TestScenarios(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestDir(d)
	Convey("it should return list of scenarios", t, func() {
		scenarios, err := GetTestScenarios(h)
		So(err, ShouldBeNil)
		_, ok := scenarios["sampleScenario"]
		So(ok, ShouldBeTrue)
		_, ok = scenarios["foo"]
		So(ok, ShouldBeFalse)
	})
	Convey("it should return list of scenarios in a role", t, func() {
		scenarios, err := GetTestScenarioRoles(h, "sampleScenario")
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", scenarios), ShouldEqual, `[listener speaker]`)
	})
}

func TestLoadTestFiles(t *testing.T) {
	d, _, h := SetupTestChain("test")
	defer CleanupTestDir(d)

	Convey("it should fail if there's no test data", t, func() {
		tests, err := LoadTestFiles(d)
		So(tests, ShouldBeNil)
		So(err.Error(), ShouldEqual, "no test files found in: "+d)
	})

	Convey("it should load test files", t, func() {
		path := h.TestPath()
		tests, err := LoadTestFiles(path)
		So(err, ShouldBeNil)
		So(len(tests), ShouldEqual, 2)
	})
}
