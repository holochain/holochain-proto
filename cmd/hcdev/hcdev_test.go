package main

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/metacurrency/holochain/cmd"
	. "github.com/smartystreets/goconvey/convey"
	_ "github.com/urfave/cli"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupApp(t *testing.T) {
	app := setupApp()
	Convey("it should create the cli App", t, func() {
		So(app.Name, ShouldEqual, "hcdev")
	})
}

func TestGoScenario_cliCommand(t *testing.T) {
	app := setupApp()

	testCommand := []string{"hcdev", "-debug", "goScenario"}
	app.Run(testCommand)

	// collect information about the execution of the command
	mutableContext, lastRunContext = GetLastRunContext()

	Convey("run the goScenario command", t, func() {
		if debug {
			fmt.Printf("HC: hcdev_test.go: TestGoScenarioCommand: mutableContext\n\n%v", spew.Sdump(mutableContext))
		}
		So(mutableContext.str["command"], ShouldEqual, "goScenario")
	})
}

func Test_testScenarioWriteEnvironment(t *testing.T) {

}

func TestGoScenario_RunScenarioTest(t *testing.T) {
	app := setupApp()

	Convey("try to build holochain without actual source", t, func() {
		// fails because there is no holochain app here
		testCommand := []string{"hcdev", "-debug", "goScenario"}
		err := app.Run(testCommand)
		// collect information about the execution of the command
		So(err, ShouldBeError)
	})

	Convey("get the scenario directory listing for one of the example apps", t, func() {
		// connect to an actual app to work with
		clutterDir, err := cmd.GolangHolochainDir("examples", "clutter")
		So(cmd.DirExists(clutterDir), ShouldEqual, true)
		if debug {
			fmt.Printf("HC: hcdev_test.go: TestGoScenario_ReadScenarioDirectory: clutterDir: %v", clutterDir)
		}

		execDir, err := cmd.MakeTmpDir("hcdev_test.go/initialise")
		So(err, ShouldBeNil)

		os.Setenv("DEBUG", "true")

		// point goScenario some app (clutterDir) and set up a working directory for the test (execDir)
		testCommand := []string{"hcdev", "-debug", "-path", clutterDir, "-execpath", execDir, "goScenario", "followAndShare"}
		//testCommand := []string{"hcdev", "-debug", "-path", clutterDir, "-execpath", execDir, "test"}
		So(err, ShouldBeNil)
		err = app.Run(testCommand)
		So(err, ShouldBeNil)

		// check that followAndShare directory is confirmed
		So(mutableContext.str["testScenarioName"], ShouldEqual, "followAndShare")

		if debug {
			fmt.Printf("HC: hcdev_test.go: TestGoScenario_ReadScenarioDirectory: mutableContext\n\n%v", spew.Sdump(mutableContext))
		}

	})

	Convey("test incorrect user inputs", t, func() {
	})
}

func TestInit(t *testing.T) {
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
	})

}
