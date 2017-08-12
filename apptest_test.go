package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"path/filepath"
	"testing"
)

func TestTestStringReplacements(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	CleanupTestDir(d)
	var lastMatches = [3][]string{{"complete match", "1st submatch", "2nd submatch"}}

	Convey("it should replace %dna%", t, func() {
		input := "%dna%"
		output := h.TestStringReplacements(input, "", "", "", &lastMatches)
		So(output, ShouldEqual, h.dnaHash.String())
	})

	Convey("it should replace %m%", t, func() {
		input := "%m1.2%"
		output := h.TestStringReplacements(input, "", "", "", &lastMatches)
		So(output, ShouldEqual, "2nd submatch")
	})

}

func TestTest(t *testing.T) {
	d, _, h := setupTestChain("test")
	CleanupTestDir(filepath.Join(d, ".holochain", "test", "test")) // delete the test data created by gen dev
	if os.Getenv("DEBUG") != "1" {
		h.config.Loggers.TestPassed.Enabled = false
		h.config.Loggers.TestFailed.Enabled = false
		h.config.Loggers.TestInfo.Enabled = false
	}
	Convey("it should fail if there's no test data", t, func() {
		err := h.Test()
		So(err[0].Error(), ShouldEqual, "open "+filepath.Join(h.rootPath, ChainTestDir)+": no such file or directory")
	})
	CleanupTestDir(d)

	d, _, h = setupTestChain("test")
	defer CleanupTestDir(d)
	if os.Getenv("DEBUG") != "1" {
		h.config.Loggers.TestPassed.Enabled = false
		h.config.Loggers.TestFailed.Enabled = false
		h.config.Loggers.TestInfo.Enabled = false
	}
	Convey("it should validate on test data", t, func() {
		err := h.Test()
		So(err, ShouldBeNil)
	})
	Convey("it should reset the database state and thus run correctly twice", t, func() {
		err := h.Test()
		So(err, ShouldBeNil)
	})

	Convey("it should fail the test on incorrect input types", t, func() {
		os.Remove(filepath.Join(d, ".holochain", "test", "test", "test_0.json"))
		err := writeFile([]byte(`[{"Zome":"zySampleZome","FnName":"addEven","Input":2,"Output":"%h%","Err":""}]`), d, ".holochain", "test", "test", "test_0.json")
		So(err, ShouldBeNil)
		err = h.Test()[0]
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "Input was not an expected type: float64")
	})
	Convey("it should fail the test on incorrect data", t, func() {
		os.Remove(filepath.Join(d, ".holochain", "test", "test", "test_0.json"))
		err := writeFile([]byte(`[{"Zome":"zySampleZome","FnName":"addEven","Input":"2","Output":"","Err":"bogus error"}]`), d, ".holochain", "test", "test", "test_0.json")
		So(err, ShouldBeNil)
		err = h.Test()[0]
		So(err, ShouldNotBeNil)
		//So(err.Error(), ShouldEqual, "Test: test_0:0\n  Expected Error: bogus error\n  Got: nil\n")
		So(err.Error(), ShouldEqual, "bogus error")
	})
}

func TestTestOne(t *testing.T) {
	d, _, h := setupTestChain("test")
	defer CleanupTestDir(d)
	if os.Getenv("DEBUG") != "" {
		h.config.Loggers.TestPassed.Enabled = false
		h.config.Loggers.TestFailed.Enabled = false
		h.config.Loggers.TestInfo.Enabled = false
	}
	Convey("it should validate on test data", t, func() {

		ShouldLog(&h.config.Loggers.TestInfo, `========================================
Test: 'test_0' starting...
========================================
Test 'test_0.0' t+0ms: { zySampleZome addEven 2 %h%   0s  false}
`, func() {
			err := h.TestOne("test_0")
			So(err, ShouldBeNil)
		})
	})
}

func TestScenarios(t *testing.T) {
	d, _, h := setupTestChain("test")
	defer CleanupTestDir(d)
	Convey("it should return list of scenarios", t, func() {
		scenarios, err := h.GetTestScenarios()
		So(err, ShouldBeNil)
		_, ok := scenarios["authorize"]
		So(ok, ShouldBeTrue)
		_, ok = scenarios["fail"]
		So(ok, ShouldBeTrue)
	})
	Convey("it should return list of scenarios in a role", t, func() {
		scenarios, err := h.GetTestScenarioRoles("authorize")
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", scenarios), ShouldEqual, `[requester responder]`)
	})
}
