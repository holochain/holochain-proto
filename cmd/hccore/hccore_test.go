package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/metacurrency/holochain/cmd"

	. "github.com/smartystreets/goconvey/convey"
	_ "github.com/urfave/cli"
	"io/ioutil"
	"testing"
)

func TestSetupApp(t *testing.T) {
	app := setupApp()
	Convey("it should create the cli App", t, func() {
		So(app.Name, ShouldEqual, "hccore")
	})
}

// OK. Im not sure how to get the pipe to the stdin of the thing. This might work no idea

func TestParadigm(t *testing.T) {

	SkipConvey("it should open a terminal in the development space, and then exit it, and check that that all happened", t, func() {

		os.Args = []string{"hccore", "paradigm"}
		fmt.Printf("hccore_test.go: Test_FromLocalFilesystem_install: os.Args: %v\n", os.Args)

		app := setupApp()
		app.Run(os.Args)

		io.WriteString(os.Stdin, "exit\n")

		//Check here if.. somrething happened
	})
}

// args app.Run must start with command name
// this test works, but it really is dangerous because the test causes the code that it's testing
// to change on the fly.  I don't think it really should work that way.

func Test_FromLocalFilesystem_install(t *testing.T) {
	SkipConvey("it should do a bunch of crazy copying around which results in a new file existing in the future and then not existing again", t, func() {

		tmpHolochainCopyDir, err := ioutil.TempDir("", "holochain.testing.hccore")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(tmpHolochainCopyDir)

		err = cmd.OsExecSilent("cp", "-r", cmd.GolangHolochainDir(), tmpHolochainCopyDir)
		if err != nil {
			fmt.Printf("  Error: cp: %v\n", err)
		}
		tmpHolochainCopyDir = filepath.Join(tmpHolochainCopyDir, "holochain")

		cmd.OsExecSilent("touch", filepath.Join(tmpHolochainCopyDir, "testing.hccore_test.go.Test_FromLocalFilesystem_install"))

		os.Args = []string{"hccore", "fromLocalFilesystem", "--sourceDirectory", tmpHolochainCopyDir, "install", "-noQuestions", "-compile", "hcdev"}
		fmt.Printf("hccore_test.go: Test_FromLocalFilesystem_install: os.Args: %v\n", os.Args)

		app := setupApp()
		app.Run(os.Args)

		testSuccessFile := cmd.GolangHolochainDir("testing.hccore_test.go.Test_FromLocalFilesystem_install")
		So(cmd.IsFile(testSuccessFile), ShouldEqual, true)

		os.Remove(testSuccessFile)
	})
}
