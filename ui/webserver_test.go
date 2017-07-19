package ui

import (
	. "github.com/metacurrency/holochain"
	. "github.com/smartystreets/goconvey/convey"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	InitializeHolochain()
	os.Exit(m.Run())
}

func TestWebServer(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)
	go NewWebServer(h, "31415").Start()
	time.Sleep(time.Second * 1)
	Convey("it should should get nothing", t, func() {
		resp, err := http.Get("http://0.0.0.0:31415")
		So(err, ShouldBeNil)
		defer resp.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		So(string(b), ShouldEqual, SampleHTML)
	})
}
