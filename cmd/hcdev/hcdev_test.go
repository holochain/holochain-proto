package main

import (
	"bytes"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	holo "github.com/metacurrency/holochain"
	"github.com/metacurrency/holochain/cmd"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/urfave/cli"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetupApp(t *testing.T) {
	app := setupApp()
	Convey("it should create the cli App", t, func() {
		So(app.Name, ShouldEqual, "hcdev")
	})
}

func TestGoScenario_cliCommand(t *testing.T) {
	os.Setenv("HCDEV_TESTING", "true")
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
	//defer os.RemoveAll(tmpTestDir)

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
	os.Setenv("HCDEV_TESTING", "true")
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

	Convey("'init bar --cloneExample=clutter' should copy files from github", t, func() {
		err = os.Chdir(tmpTestDir)
		if err != nil {
			panic(err)
		}

		// it should clone with the same name as the repo
		os.Args = []string{"hcdev", "init", "-cloneExample=clutter"}
		err = app.Run(os.Args)
		So(err, ShouldBeNil)
		So(cmd.IsFile(filepath.Join(tmpTestDir, "clutter", "dna", "clutter", "clutter.js")), ShouldBeTrue)

		// or with a specified name
		err = os.Chdir(tmpTestDir)
		if err != nil {
			panic(err)
		}
		os.Args = []string{"hcdev", "init", "-cloneExample=clutter", "myClutter"}
		err = app.Run(os.Args)
		So(err, ShouldBeNil)
		So(cmd.IsFile(filepath.Join(tmpTestDir, "myClutter", "dna", "clutter", "clutter.js")), ShouldBeTrue)
		So(cmd.IsDir(tmpTestDir, holo.ChainDataDir), ShouldBeFalse)

		// but fail if the directory is already there
		err = os.Chdir(tmpTestDir)
		if err != nil {
			panic(err)
		}
		os.Args = []string{"hcdev", "init", "-cloneExample=clutter"}
		err = app.Run(os.Args)
		So(err, ShouldNotBeNil)
		So(os.Getenv("HCDEV_TESTING_EXITERR"), ShouldEqual, "1")
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
	Convey("'package' should print a scaffold file to stdout", t, func() {
		scaffold, err := runAppWithStdoutCapture(app, []string{"hcdev", "package"})
		So(err, ShouldBeNil)
		So(scaffold, ShouldContainSubstring, fmt.Sprintf(`"ScaffoldVersion": "%s"`, holo.ScaffoldVersion))
	})
	app = setupApp()
	d := holo.SetupTestDir()
	defer os.RemoveAll(d)
	Convey("'package' should output a scaffold file to file", t, func() {
		runAppWithStdoutCapture(app, []string{"hcdev", "package", filepath.Join(d, "scaff.json")})
		scaffold, err := holo.ReadFile(d, "scaff.json")
		So(err, ShouldBeNil)
		So(string(scaffold), ShouldContainSubstring, fmt.Sprintf(`"ScaffoldVersion": "%s"`, holo.ScaffoldVersion))

	})
}

func TestWeb(t *testing.T) {
	os.Setenv("HCDEV_TESTING", "true")
	tmpTestDir, app := setupTestingApp("foo")
	defer os.RemoveAll(tmpTestDir)

	Convey("'web' should run a webserver", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcdev", "web"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "on port:4141")
		So(out, ShouldContainSubstring, "Serving holochain with DNA hash:")
	})
	app = setupApp()

	Convey("'web' not in an app directory should produce error", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcdev", "-path", tmpTestDir, "web"})
		So(err, ShouldBeError)
		So(out, ShouldContainSubstring, "doesn't look like a holochain app")
	})
}

func runAppWithStdoutCapture(app *cli.App, args []string) (out string, err error) {
	os.Args = args

	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() { err = app.Run(os.Args) }()
	time.Sleep(time.Second / 2)

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	out = <-outC
	return
}

func setupTestingApp(name string) (string, *cli.App) {
	tmpTestDir, err := ioutil.TempDir("", "holochain.testing.hcdev")
	if err != nil {
		panic(err)
	}

	err = os.Chdir(tmpTestDir)
	if err != nil {
		panic(err)
	}
	app := setupApp()

	os.Args = []string{"hcdev", "init", "-test", name}
	err = app.Run(os.Args)
	if err != nil {
		panic(err)
	}

	err = os.Chdir(filepath.Join(tmpTestDir, name))
	if err != nil {
		panic(err)
	}
	return tmpTestDir, app
}
