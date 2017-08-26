package main

import (
	"bytes"
	"fmt"
	holo "github.com/metacurrency/holochain"
	cmd "github.com/metacurrency/holochain/cmd"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/urfave/cli"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSetupApp(t *testing.T) {
	app := setupApp()
	Convey("it should create the cli App", t, func() {
		So(app.Name, ShouldEqual, "hcadmin")
	})
}

func TestInit(t *testing.T) {
	d := holo.SetupTestDir()
	defer os.RemoveAll(d)
	app := setupApp()
	Convey("before init it should return service not initialized error", t, func() {
		os.Args = []string{"hadmin", "-path", d, "status"}
		err := app.Run(os.Args)
		So(err, ShouldEqual, cmd.ErrServiceUninitialized)
	})
	app = setupApp()
	Convey("after init status should show no chains", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "init", "testing-identity"})
		So(err, ShouldBeNil)
		So(out, ShouldEqual, "Holochain service initialized\n")
		app = setupApp()
		out, err = runAppWithStdoutCapture(app, []string{"hcadmin", "-verbose", "-path", d, "status"})
		So(err, ShouldBeNil)
		So(out, ShouldEqual, fmt.Sprintf("hcadmin version %s \nno installed chains\n", app.Version))
	})
}

func TestJoin(t *testing.T) {
	d := holo.SetupTestDir()
	defer os.RemoveAll(d)
	app := setupApp()
	os.Args = []string{"hadmin", "-path", d, "init", "test-identity"}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
	//	cmd := cmd.OsExecPipes("hcdev", "-path", d, "init", "-test", "testAppSrc")
	cmd := exec.Command("hcdev", "-path", d, "init", "-test", "testAppSrc")
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
	fmt.Printf("OUT:%s", string(out))

	time.Sleep(time.Millisecond * 100) // give the processes time to complete
	app = setupApp()
	Convey("it should join a chain", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-verbose", "-path", d, "join", filepath.Join(d, "testAppSrc"), "testApp"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, fmt.Sprintf("hcadmin version %s \n", app.Version))
		So(out, ShouldContainSubstring, fmt.Sprintf("joined testApp from %s/testAppSrc", d))
		So(out, ShouldContainSubstring, "Genesis entries added and DNA hashed for new holochain with ID:")
	})
	app = setupApp()
	Convey("after join status should show it", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "status"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "installed holochains:\n    testApp Qm")
	})
	app = setupApp()
	Convey("after join dump -chain should show it", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "dump", "-chain", "testApp"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "Chain for: Qm")
	})
	app = setupApp()
	Convey("after join dump -dht should show it", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "dump", "-dht", "testApp"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "DHT for: Qm")
		So(out, ShouldContainSubstring, "DHT changes:2")
	})
}

func TestBridge(t *testing.T) {
	d := holo.SetupTestDir()
	defer os.RemoveAll(d)
	app := setupApp()
	_, err := runAppWithStdoutCapture(app, []string{"hadmin", "-path", d, "init", "test-identity"})
	if err != nil {
		panic(err)
	}

	// create two different instances of the testing app (i.e. with different dna) and join them both
	cmd.OsExecPipes("hcdev", "-path", d, "init", "-test", "testAppSrc1")
	cmd.OsExecPipes("hcdev", "-path", d, "init", "-test", "testAppSrc2")
	time.Sleep(time.Millisecond * 100) // give the processes time to complete
	app = setupApp()
	_, err = runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "join", filepath.Join(d, "testAppSrc1"), "testApp1"})
	if err != nil {
		panic(err)
	}
	app = setupApp()
	_, err = runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "join", filepath.Join(d, "testAppSrc2"), "testApp2"})
	if err != nil {
		panic(err)
	}

	Convey("it should bridge chains", t, func() {
		app = setupApp()
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "bridge", "testApp1", "testApp2", "-bridgeToAppData", "some to app data"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "bridge genesis to-- other side is:QmaYyhRCLoC2JToVT5CdCUV4eNvdYKu867uYXWpFbBZLry bridging data:some to app data\n")
	})
	Convey("after bridge status should show it", t, func() {
		app = setupApp()
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "status"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "testApp1 QmaYyhRCLoC2JToVT5CdCUV4eNvdYKu867uYXWpFbBZLry\n        bridged to: QmeZphnoyd2Hwqatx8RVGMBGF3st4Gt1S92sZx2ZajUfHY")
		So(out, ShouldContainSubstring, "testApp2 QmeZphnoyd2Hwqatx8RVGMBGF3st4Gt1S92sZx2ZajUfHY\n        bridged from by token:")
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
