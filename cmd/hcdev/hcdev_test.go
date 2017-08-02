package main

import (
	// flag     "flag"
	fmt			 "fmt"
	spew			"github.com/davecgh/go-spew/spew"

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
		So(mutableContext["command"], ShouldEqual, "goScenario")
	})
}

