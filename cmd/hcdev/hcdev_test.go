package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	holo "github.com/HC-Interns/holochain-proto"
	. "github.com/HC-Interns/holochain-proto/apptest"
	"github.com/HC-Interns/holochain-proto/cmd"
	. "github.com/HC-Interns/holochain-proto/hash"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/urfave/cli"
)

func TestMain(m *testing.M) {
	// disable UPNP for tests
	os.Setenv("HOLOCHAINCONFIG_ENABLENATUPNP", "false")
	holo.InitializeHolochain()
	os.Exit(m.Run())
}

func TestSetupApp(t *testing.T) {
	app := setupApp()
	Convey("it should create the cli App", t, func() {
		So(app.Name, ShouldEqual, "hcdev")
	})
}

func TestDump(t *testing.T) {
	d, s, h := holo.PrepareTestChain("test")
	defer holo.CleanupTestChain(h, d)
	app := setupApp()

	Convey("'dump --chain' should show chain entries as a human readable string", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcdev", "-DHTport=6001", "-execpath", s.Path, "-path", "test", "dump", "--chain"})

		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "%dna:")
		So(out, ShouldContainSubstring, "%agent:")
	})

	Convey("'dump --dht' should show chain entries as a human readable string", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcdev", "-DHTport=6001", "-execpath", s.Path, "-path", "test", "dump", "--dht"})

		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "DHT changes: 2")
		So(out, ShouldContainSubstring, "DHT entries:")
	})

	Convey("'dump --chain --json' should show chain entries as JSON string", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcdev", "-DHTport=6001", "-execpath", s.Path, "-path", "test", "dump", "--chain", "--json"})

		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "{\n    \"%dna\": {")
		So(out, ShouldContainSubstring, ",\n    \"%agent\": {")
	})

	Convey("'dump --dht --json' should show chain entries as JSON string", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcdev", "-DHTport=6001", "-execpath", s.Path, "-path", "test", "dump", "--dht", "--json"})

		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "\"dht_changes\": [")
		So(out, ShouldContainSubstring, "\"dht_entries\": [")
	})

	Convey("'dump --chain --format string' should show chain entries as a human readable string", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcdev", "-DHTport=6001", "-execpath", s.Path, "-path", "test", "dump", "--chain", "--format", "string"})

		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "%dna:")
		So(out, ShouldContainSubstring, "%agent:")
	})

	Convey("'dump --chain --format dot' should show chain entries as GraphViz DOT format", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcdev", "-DHTport=6001", "-execpath", s.Path, "-path", "test", "dump", "--chain", "--format", "dot"})

		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "digraph chain {")
	})
	os.Unsetenv("HOLOCHAINCONFIG_ENABLENATUPNP")
}

func TestGoScenario_cliCommand(t *testing.T) {
	os.Setenv("HC_TESTING", "true")
	app := setupApp()

	testCommand := []string{"hcdev", "-debug", "scenario"}
	app.Run(testCommand)

	// collect information about the execution of the command
	mutableContext, lastRunContext = GetLastRunContext()

	Convey("run the scenario command", t, func() {
		if debug {
			fmt.Printf("HC: hcdev_test.go: TestGoScenarioCommand: mutableContext\n\n%v", spew.Sdump(mutableContext))
		}
		So(mutableContext.str["command"], ShouldEqual, "scenario")
	})
}

func Test_testScenarioWriteEnvironment(t *testing.T) {

}

func TestRunScenarioTest(t *testing.T) {
	app := setupApp()

	Convey("try to build holochain without actual source", t, func() {
		// fails because there is no holochain app here
		testCommand := []string{"hcdev", "-debug", "scenario"}
		err := app.Run(testCommand)
		// collect information about the execution of the command
		So(err, ShouldBeError)
	})

	tmpTestDir, app := setupTestingApp("foo")
	defer os.RemoveAll(tmpTestDir)

	Convey("run the scenario in the testing app", t, func() {

		execDir, err := cmd.MakeTmpDir("hcdev_scenariotest")
		So(err, ShouldBeNil)
		//defer os.RemoveAll(execDir)  // can't delete because this stuff runs in the background...

		// setupTestingApp moved us into the app so we just need to point to  a working directory for the test (execDir)
		testCommand := []string{"hcdev", "-path", tmpTestDir + "/foo", "-execpath", execDir, "scenario", "sampleScenario"}

		err = app.Run(testCommand)
		So(err, ShouldBeNil)

		// check that scenario directory is confirmed
		So(mutableContext.str["testScenarioName"], ShouldEqual, "sampleScenario")

		if debug {
			fmt.Printf("HC: hcdev_test.go: TestRunScenarioTest: mutableContext\n\n%v", spew.Sdump(mutableContext))
		}

	})

	//Convey("test incorrect user inputs", t, func() {
	//})
}

func TestInit(t *testing.T) {
	os.Setenv("HC_TESTING", "true")
	tmpTestDir, err := ioutil.TempDir("", "holochain.testing.hcdev")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpTestDir)

	err = os.Chdir(tmpTestDir)
	if err != nil {
		panic(err)
	}

	app := setupApp()
	Convey("'init foo' should create default files in 'foo' directory", t, func() {
		os.Args = []string{"hcdev", "init", "foo"}
		err = app.Run(os.Args)
		So(err, ShouldBeNil)
		So(cmd.IsFile(filepath.Join(tmpTestDir, "foo", "dna", "dna.json")), ShouldBeTrue)
		So(cmd.IsDir(tmpTestDir, holo.ChainDataDir), ShouldBeFalse)
	})

	Convey("'init /tmp/foo' should create default files in the absolute '/tmp/foo' directory", t, func() {
		tmpFoo := filepath.Join("/tmp", "foo")
		os.Args = []string{"hcdev", "init", tmpFoo}
		err = app.Run(os.Args)
		So(err, ShouldBeNil)
		So(cmd.IsFile(filepath.Join(tmpFoo, "dna", "dna.json")), ShouldBeTrue)
		So(cmd.IsDir(tmpTestDir, holo.ChainDataDir), ShouldBeFalse)
		os.RemoveAll(tmpFoo)
	})

	Convey("'init bar --clone foo' should copy files from foo to bar", t, func() {
		p := filepath.Join(tmpTestDir, "foo", "ui", "foo.js")
		f, err := os.Create(p)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		err = os.Chdir(tmpTestDir)
		if err != nil {
			panic(err)
		}

		os.Args = []string{"hcdev", "init", "-clone", "foo", "bar"}

		err = app.Run(os.Args)
		So(err, ShouldBeNil)
		So(cmd.IsFile(filepath.Join(tmpTestDir, "bar", "ui", "foo.js")), ShouldBeTrue)
		So(cmd.IsDir(tmpTestDir, holo.ChainDataDir), ShouldBeFalse)
	})

	Convey("'init bar --cloneExample=HoloWorld' should copy files from github", t, func() {
		err = os.Chdir(tmpTestDir)
		if err != nil {
			panic(err)
		}

		// it should clone with the same name as the repo
		os.Args = []string{"hcdev", "init", "-cloneExample=HoloWorld"}
		err = app.Run(os.Args)
		So(err, ShouldBeNil)
		So(cmd.IsFile(filepath.Join(tmpTestDir, "HoloWorld", "dna", "dna.json")), ShouldBeTrue)
		// or from a branch
		err = os.Chdir(tmpTestDir)
		if err != nil {
			panic(err)
		}
		os.Args = []string{"hcdev", "init", "-cloneExample=HoloWorld", "-fromDevelop", "HoloWorld2"}
		err = app.Run(os.Args)
		So(err, ShouldBeNil)
		So(cmd.IsFile(filepath.Join(tmpTestDir, "HoloWorld2", "dna", "dna.json")), ShouldBeTrue)

		// or with a specified name
		err = os.Chdir(tmpTestDir)
		if err != nil {
			panic(err)
		}
		os.Args = []string{"hcdev", "init", "-cloneExample=HoloWorld", "myHoloWorld"}
		err = app.Run(os.Args)
		So(err, ShouldBeNil)
		So(cmd.IsFile(filepath.Join(tmpTestDir, "myHoloWorld", "dna", "dna.json")), ShouldBeTrue)
		So(cmd.IsDir(tmpTestDir, holo.ChainDataDir), ShouldBeFalse)

		// but fail if the directory is already there
		err = os.Chdir(tmpTestDir)
		if err != nil {
			panic(err)
		}
		os.Args = []string{"hcdev", "init", "-cloneExample=HoloWorld"}
		err = app.Run(os.Args)
		So(err, ShouldNotBeNil)
		So(os.Getenv("HC_TESTING_EXITERR"), ShouldEqual, "1")
	})

	Convey("'init -test testingApp' should create the test app", t, func() {
		err = os.Chdir(tmpTestDir)
		if err != nil {
			panic(err)
		}

		os.Args = []string{"hcdev", "init", "-test", "testingApp"}

		err = app.Run(os.Args)
		So(err, ShouldBeNil)
		So(cmd.IsFile(filepath.Join(tmpTestDir, "testingApp", "dna", "jsSampleZome", "jsSampleZome.js")), ShouldBeTrue)
		So(cmd.IsDir(tmpTestDir, holo.ChainDataDir), ShouldBeFalse)
	})

}

func TestPackage(t *testing.T) {
	app := setupApp()
	tmpTestDir, app := setupTestingApp("foo")
	defer os.RemoveAll(tmpTestDir)
	Convey("'package' should print a appPackage file to stdout", t, func() {
		appPackage, err := cmd.RunAppWithStdoutCapture(app, []string{"hcdev", "package"}, 2*time.Second)
		So(err, ShouldBeNil)
		So(appPackage, ShouldContainSubstring, fmt.Sprintf(`"Version": "%s"`, holo.AppPackageVersion))
	})
	app = setupApp()
	d := holo.SetupTestDir()
	defer os.RemoveAll(d)
	Convey("'package' should output an appPackage file to a file", t, func() {
		cmd.RunAppWithStdoutCapture(app, []string{"hcdev", "package", filepath.Join(d, "scaff.json")}, 2*time.Second)
		appPackage, err := holo.ReadFile(d, "scaff.json")
		So(err, ShouldBeNil)
		So(string(appPackage), ShouldContainSubstring, fmt.Sprintf(`"Version": "%s"`, holo.AppPackageVersion))

	})
}

func TestWeb(t *testing.T) {
	os.Setenv("HC_TESTING", "true")
	tmpTestDir, app := setupTestingApp("foo")
	defer os.RemoveAll(tmpTestDir)

	os.Unsetenv("HCLOG_DHT_ENABLE")
	os.Unsetenv("HCLOG_GOSSIP_ENABLE")
	os.Unsetenv("HCLOG_DEBUG_ENABLE")

	Convey("'web' should run a webserver", t, func() {
		out, err := cmd.RunAppWithStdoutCapture(app, []string{"hcdev", "-upnp=false", "web"}, 5*time.Second)
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "on port:4141")
		So(out, ShouldContainSubstring, "Serving holochain with DNA hash:")
		// it should include app level debug but not holochain debug
		So(out, ShouldContainSubstring, "running zyZome genesis")
		So(out, ShouldNotContainSubstring, "NewEntry of %dna added as: Qm")
	})
	app = setupApp()

	Convey("'web -debug' should run a webserver and include holochain debug info", t, func() {
		out, err := cmd.RunAppWithStdoutCapture(app, []string{"hcdev", "-debug", "-upnp=false", "web", "4142"}, 5*time.Second)
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "running zyZome genesis")
		So(out, ShouldContainSubstring, "NewEntry of %dna added as: Qm")
	})
	app = setupApp()

	Convey("'web' not in an app directory should produce error", t, func() {
		out, err := cmd.RunAppWithStdoutCapture(app, []string{"hcdev", "-path", tmpTestDir, "web"}, 1*time.Second)
		So(err, ShouldBeError)
		So(out, ShouldContainSubstring, "doesn't look like a holochain app")
	})
}

func TestIdenity(t *testing.T) {
	os.Setenv("HC_TESTING", "true")
	tmpTestDir, app := setupTestingApp("foo")
	defer os.RemoveAll(tmpTestDir)
	Convey("it should create default from users config", t, func() {
		host, _ := os.Hostname()
		So(getIdentity("", ""), ShouldEqual, sysUser.Username+"@"+host)
	})
	Convey("it should use params", t, func() {
		So(getIdentity("foo", "bar"), ShouldEqual, "foo@bar")
	})
	Convey("verbose should show the identity and nodeid", t, func() {
		out, err := cmd.RunAppWithStdoutCapture(app, []string{"hcdev", "-agentID=foo", "-serverID=bar", "-verbose", "web"}, 1*time.Second)
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "Identity: foo")
		So(out, ShouldContainSubstring, "NodeID: QmNQq6JDkxoYFzWVi5C4fVQ47zbFpUDiRg2AF8XE6CDDow")
	})
}

func TestBridging(t *testing.T) {
	os.Setenv("HC_TESTING", "true")
	tmpTestDir, app := setupTestingApp("foo", "bar")
	defer os.RemoveAll(tmpTestDir)

	// bridging to ourselves for this test, so write out a bridgeSpecFile to use
	bridgeSourceDir := filepath.Join(tmpTestDir, "bar")
	data := []BridgeSpec{BridgeSpec{Path: bridgeSourceDir, Side: holo.BridgeCallee, BridgeZome: "jsSampleZome", BridgeGenesisCalleeData: "some data 314"}}
	var b bytes.Buffer
	err := holo.Encode(&b, "json", data)

	if err != nil {
		panic(err)
	}
	if err := holo.WriteFile(b.Bytes(), tmpTestDir, "specs.json"); err != nil {
		panic(err)
	}

	Convey("bridgeSpecsFile path should setup bridging", t, func() {
		out, err := cmd.RunAppWithStdoutCapture(app, []string{"hcdev", "-bridgeSpecs", filepath.Join(tmpTestDir, "specs.json"), "-upnp=false", "web"}, 5*time.Second)
		So(err, ShouldBeNil)
		//So(out, ShouldContainSubstring, fmt.Sprintf("bridging to %s using zome: jsSampleZome", bridgeSourceDir))
		So(out, ShouldContainSubstring, fmt.Sprintf("Copying bridge chain bar to:"))
		So(out, ShouldContainSubstring, fmt.Sprintf("bridge genesis to-- other side is:")) // getting the DNA is a pain so skip it.
		So(out, ShouldContainSubstring, fmt.Sprintf("bridging data:some data 314"))
	})
}

func TestSaveBridgeApps(t *testing.T) {
	hashRed, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzfro1")
	hashBlue, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzfro2")

	bridgeAppsForTests := []BridgeAppForTests{
		BridgeAppForTests{
			BridgeApp: holo.BridgeApp{
				Name: "red fish",
				DNA:  hashRed,
				Side: holo.BridgeCaller,
				BridgeGenesisCallerData: "data red from",
				BridgeGenesisCalleeData: "data blue to",
				Port:       "1234",
				BridgeZome: "redzome",
			},
		},
		BridgeAppForTests{
			BridgeApp: holo.BridgeApp{
				Name: "blue fish",
				DNA:  hashBlue,
				Side: holo.BridgeCallee,
				BridgeGenesisCallerData: "data red from",
				BridgeGenesisCalleeData: "data blue to",
				Port:       "4321",
				BridgeZome: "bluezome",
			},
		},
	}

	Convey("you can save out bridge app data for scenario testing", t, func() {
		fileName, err := saveBridgeAppsToTmpFile(bridgeAppsForTests)
		So(err, ShouldBeNil)

		bridgeApps, err := getBridgeAppsFromTmpFile(fileName)
		So(err, ShouldBeNil)
		So(reflect.DeepEqual(bridgeApps[0], bridgeAppsForTests[0].BridgeApp), ShouldBeTrue)
		So(reflect.DeepEqual(bridgeApps[1], bridgeAppsForTests[1].BridgeApp), ShouldBeTrue)
		So(len(bridgeApps), ShouldEqual, 2)
	})
}

func TestConfigFlagsDefault(t *testing.T) {
	os.Setenv("HC_TESTING", "true")
	tmpTestDir, err := ioutil.TempDir("", "holochain.testing.hcdev")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpTestDir)

	runDir := filepath.Join(tmpTestDir, holo.DefaultDirectoryName+"dev")

	os.Setenv("HOLOPATHDEV", runDir)

	err = os.Chdir(tmpTestDir)
	if err != nil {
		panic(err)
	}

	app := setupApp()
	_, err = runAppWithStdoutCapture(app, []string{"hcdev", "init", "foo"})

	Convey("defaults are mdns on upnp off", t, func() {
		app := setupApp()
		runAppWithStdoutCapture(app, []string{"hcdev", "test"})
		var config holo.Config
		err = holo.DecodeFile(&config, filepath.Join(runDir, "foo", holo.ConfigFileName+".json"))
		So(err, ShouldBeNil)
		So(config.EnableMDNS, ShouldBeTrue)
		So(config.EnableNATUPnP, ShouldBeFalse)
	})

	os.Unsetenv("HOLOPATHDEV")
	os.Unsetenv("HC_TESTING")
}

func setupTestingApp(names ...string) (string, *cli.App) {
	tmpTestDir, err := ioutil.TempDir("", "holochain.testing.hcdev")
	if err != nil {
		panic(err)
	}

	app := setupApp()

	for _, name := range names {
		_setupTestingApp(name, app, tmpTestDir)
	}

	err = os.Chdir(filepath.Join(tmpTestDir, names[0]))
	if err != nil {
		panic(err)
	}
	return tmpTestDir, app
}

func _setupTestingApp(name string, app *cli.App, tmpTestDir string) {
	err := os.Chdir(tmpTestDir)
	if err != nil {
		panic(err)
	}

	os.Args = []string{"hcdev", "init", "-test", name}
	err = app.Run(os.Args)
	if err != nil {
		panic(err)
	}

	// cleanup env flags set
	os.Unsetenv("HOLOCHAINCONFIG_ENABLENATUPNP")
	os.Unsetenv("HOLOCHAINCONFIG_BOOTSTRAP")
	os.Unsetenv("HOLOCHAINCONFIG_ENABLEMDNS")
}

func runAppWithStdoutCapture(app *cli.App, args []string) (out string, err error) {
	return cmd.RunAppWithStdoutCapture(app, args, time.Second*5)
}
