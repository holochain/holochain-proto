package main

import (
	"bytes"
	"github.com/metacurrency/holochain/cmd"
	. "github.com/smartystreets/goconvey/convey"
	_ "github.com/urfave/cli"
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

}

func TestWeb(t *testing.T) {
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

	os.Args = []string{"hcdev", "init", "foo"}
	err = app.Run(os.Args)
	if err != nil {
		panic(err)
	}

	err = os.Chdir(filepath.Join(tmpTestDir, "foo"))
	if err != nil {
		panic(err)
	}

	Convey("'web' should run a webserver", t, func() {
		os.Args = []string{"hcdev", "web"}

		old := os.Stdout // keep backup of the real stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		go app.Run(os.Args)
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
		out := <-outC
		So(out, ShouldContainSubstring, "on port:4141")
		So(out, ShouldContainSubstring, "Serving holochain with DNA hash:")
	})
}
