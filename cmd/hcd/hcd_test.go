package main

import (
	"bytes"
	holo "github.com/metacurrency/holochain"
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
		So(app.Name, ShouldEqual, "hcd")
	})
}

func TestWeb(t *testing.T) {
	holo.InitializeHolochain()
	tmpTestDir, err := ioutil.TempDir("", "holochain.testing.hcdev")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpTestDir)

	err = os.Chdir(tmpTestDir)
	if err != nil {
		panic(err)
	}

	agent := "Fred Flintstone <fred@flintstone.com>"
	s, err := holo.Init(filepath.Join(tmpTestDir, holo.DefaultDirectoryName), holo.AgentIdentity(agent), nil)
	if err != nil {
		panic(err)
	}
	root := filepath.Join(s.Path, "testApp")
	h, err := s.MakeTestingApp(root, "json", holo.InitializeDB, holo.CloneWithNewUUID, nil)
	if err != nil {
		panic(err)
	}
	h.GenChain()
	app := setupApp()

	Convey("it should run a webserver", t, func() {
		os.Args = []string{"hcd", "-path", s.Path, "-verbose", "testApp"}

		old := os.Stdout // keep backup of the real stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		go app.Run(os.Args)
		time.Sleep(time.Second)

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

		So(out, ShouldContainSubstring, h.DNAHash().String()+" on port 3141")
		So(out, ShouldContainSubstring, "Serving holochain with DNA hash:")
	})
}
