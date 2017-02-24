package main

import (
	. "github.com/smartystreets/goconvey/convey"
	_ "github.com/urfave/cli"
	"testing"
)

func TestSetupApp(t *testing.T) {
	app := setupApp()
	Convey("it should create the cli App", t, func() {
		So(app.Name, ShouldEqual, "hc")
	})
}
