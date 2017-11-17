package main

import (
	holo "github.com/metacurrency/holochain"
	"github.com/metacurrency/holochain/cmd"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/urfave/cli"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	holo.InitializeHolochain()
	os.Exit(m.Run())
}

func TestSetupApp(t *testing.T) {
	app := setupApp()
	Convey("it should create the cli App", t, func() {
		So(app.Name, ShouldEqual, "hcd")
	})
}

func TestWeb(t *testing.T) {
	tmpTestDir, s, h, app := setupTestingApp("testApp")
	defer os.RemoveAll(tmpTestDir)

	Convey("it should run a webserver", t, func() {
		out, err := cmd.RunAppWithStdoutCapture(app, []string{"hcd", "-path", s.Path, "testApp"}, 5*time.Second)
		So(err, ShouldBeNil)

		So(out, ShouldContainSubstring, h.DNAHash().String()+" on port 3141")
		So(out, ShouldContainSubstring, "Serving holochain with DNA hash:")
		So(out, ShouldNotContainSubstring, "running zyZome genesis")
	})
}

func setupTestingApp(name string) (string, *holo.Service, *holo.Holochain, *cli.App) {
	tmpTestDir, err := ioutil.TempDir("", "holochain.testing.hcd")
	if err != nil {
		panic(err)
	}

	err = os.Chdir(tmpTestDir)
	if err != nil {
		panic(err)
	}

	agent := "Fred Flintstone <fred@flintstone.com>"
	s, err := holo.Init(filepath.Join(tmpTestDir, holo.DefaultDirectoryName), holo.AgentIdentity(agent), nil)
	if err != nil {
		panic(err)
	}
	root := filepath.Join(s.Path, name)
	h, err := s.MakeTestingApp(root, "json", holo.InitializeDB, holo.CloneWithNewUUID, nil)
	if err != nil {
		panic(err)
	}
	h.GenChain()
	app := setupApp()
	return tmpTestDir, s, h, app
}
