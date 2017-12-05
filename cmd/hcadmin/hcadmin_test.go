package main

import (
	"fmt"
	holo "github.com/metacurrency/holochain"
	"github.com/metacurrency/holochain/cmd"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/urfave/cli"
	"os"
	"path/filepath"
	"regexp"
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

func TestJoinFromSourceDir(t *testing.T) {
	d := holo.SetupTestDir()
	defer os.RemoveAll(d)
	app := setupApp()
	os.Args = []string{"hcadmin", "-path", d, "init", "test-identity"}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
	hcdev := filepath.Join(os.Getenv("GOPATH"), "/bin/hcdev")
	err = cmd.OsExecSilent(hcdev, "-path", d, "init", "-test", "testAppSrc")
	if err != nil {
		panic(err)
	}
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
		So(out, ShouldContainSubstring, "DHT changes: 2")
	})
}

func TestJoinFromPackage(t *testing.T) {
	d := holo.SetupTestDir()
	//	defer os.RemoveAll(d)
	app := setupApp()
	_, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "init", "test-identity"})
	if err != nil {
		panic(err)
	}

	err = holo.WriteFile([]byte(holo.BasicTemplateAppPackage), d, "appPackage."+holo.BasicTemplateAppPackageFormat)
	if err != nil {
		panic(err)
	}
	app = setupApp()
	Convey("it should join a chain", t, func() {
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-verbose", "-path", d, "join", filepath.Join(d, "appPackage."+holo.BasicTemplateAppPackageFormat), "testApp"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, fmt.Sprintf("hcadmin version %s \n", app.Version))
		So(out, ShouldContainSubstring, fmt.Sprintf("joined testApp from %s/appPackage."+holo.BasicTemplateAppPackageFormat, d))
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
		So(out, ShouldContainSubstring, "DHT changes: 2")
	})
}

func TestBridge(t *testing.T) {
	d := holo.SetupTestDir()
	defer os.RemoveAll(d)
	app := setupApp()
	_, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-path", d, "init", "test-identity"})
	if err != nil {
		panic(err)
	}

	// create two different instances of the testing app (i.e. with different dna) and join them both
	hcdev := filepath.Join(os.Getenv("GOPATH"), "/bin/hcdev")
	err = cmd.OsExecSilent(hcdev, "-path", d, "init", "-test", "testAppSrc1")
	if err != nil {
		panic(err)
	}
	err = cmd.OsExecSilent(hcdev, "-path", d, "init", "-test", "testAppSrc2")
	if err != nil {
		panic(err)
	}
	app = setupApp()
	out1, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-verbose", "-path", d, "join", filepath.Join(d, "testAppSrc1"), "testApp1"})
	if err != nil {
		panic(err)
	}
	app = setupApp()
	out2, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-verbose", "-path", d, "join", filepath.Join(d, "testAppSrc2"), "testApp2"})
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile(`new holochain with ID: (Qm.*)`)
	x := re.FindStringSubmatch(out1)
	if len(x) == 0 {
		panic("expected to find the DNA for app1 in " + out1)
	}
	testApp1DNA := x[1]
	x = re.FindStringSubmatch(out2)
	if len(x) == 0 {
		panic("expected to find the DNA for app2 in " + out2)
	}
	testApp2DNA := x[1]

	Convey("it should bridge chains", t, func() {
		app = setupApp()
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-debug", "-path", d, "bridge", "testApp1", "testApp2", "-bridgeToAppData", "some to app data"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "bridge genesis to-- other side is:"+testApp1DNA+" bridging data:some to app data\n")
	})
	Convey("after bridge status should show it", t, func() {
		app = setupApp()
		out, err := runAppWithStdoutCapture(app, []string{"hcadmin", "-debug", "-path", d, "status"})
		So(err, ShouldBeNil)
		So(out, ShouldContainSubstring, "testApp1 "+testApp1DNA+"\n        bridged to: "+testApp2DNA)
		So(out, ShouldContainSubstring, "testApp2 "+testApp2DNA+"\n        bridged from by token:")
	})
}

func runAppWithStdoutCapture(app *cli.App, args []string) (out string, err error) {
	return cmd.RunAppWithStdoutCapture(app, args, time.Second*5)
}
