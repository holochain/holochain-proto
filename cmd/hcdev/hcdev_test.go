package main

import (
	// flag     "flag"
	filepath 	"path/filepath"
	fmt			 	"fmt"
	os 				"os"
	spew			"github.com/davecgh/go-spew/spew"

	cmd 			"github.com/metacurrency/holochain/cmd"	

	.        "github.com/smartystreets/goconvey/convey"
	// cli      "github.com/urfave/cli"
	testing  "testing"
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
	app.Run(testCommand )

	// collect information about the execution of the command
	mutableContext, lastRunContext = GetLastRunContext()

	Convey("run the goScenario command", t, func() {
		if debug {
			fmt.Printf("HC: hcdev_test.go: TestGoScenarioCommand: mutableContext\n\n%v", spew.Sdump(mutableContext))
		}
		So(mutableContext.str["command"], ShouldEqual, "goScenario")
	})
}

func TestGoScenario_ReadScenarioDirectory(t *testing.T) {
	app := setupApp()

	Convey("try to build holochain without actual source", t, func() {
		// fails because there is no holochain app here
		testCommand := []string{"hcdev", "-debug", "goScenario"}
		err := app.Run(testCommand )
		// collect information about the execution of the command
		So(err, ShouldBeError)
	})

	Convey("get the scenario directory listing for one of the example apps", t, func() {
		// connect to an actual app to work with
		currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		So(err, ShouldBeNil)
		clutterDir 		:= filepath.Join(currentDir, "../..", "examples", "clutter")
		So(cmd.DirExists(clutterDir), ShouldEqual, true)

		execDir, err := cmd.MakeTmpDir("hcdev_test.go/initialise")
		So(err, ShouldBeNil)

		
		os.Setenv("DEBUG", "true")

		// point goScenario some app (clutterDir) and set up a working directory for the test (execDir)
		testCommand := []string{"hcdev", "-debug", "-path", clutterDir, "-execpath", execDir, "goScenario", "followAndShare"}
		So(err, ShouldBeNil)
		err = app.Run(testCommand )
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