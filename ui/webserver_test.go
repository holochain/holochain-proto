package ui

import (
	"bytes"
	. "github.com/HC-Interns/holochain-proto"
	. "github.com/HC-Interns/holochain-proto/hash"
	. "github.com/smartystreets/goconvey/convey"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// disable UPNP for tests
	os.Setenv("HOLOCHAINCONFIG_ENABLENATUPNP", "false")
	InitializeHolochain()
	os.Exit(m.Run())
}

func TestWebServer(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	ws := NewWebServer(h, "31415")
	ws.Start()
	time.Sleep(time.Second * 1)
	Convey("it should should return the index page", t, func() {
		resp, err := http.Get("http://0.0.0.0:31415")
		So(err, ShouldBeNil)
		defer resp.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		So(string(b), ShouldEqual, SampleHTML)
	})

	Convey("it should should fail on bad function calls", t, func() {
		resp, err := http.Get("http://0.0.0.0:31415/fn/bogus")
		So(err, ShouldBeNil)
		defer resp.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, 400)
		So(string(b), ShouldEqual, "bad request\n")
	})

	Convey("it should call functions", t, func() {
		body := bytes.NewBuffer([]byte("language"))
		resp, err := http.Post("http://0.0.0.0:31415/fn/jsSampleZome/getProperty", "", body)
		So(err, ShouldBeNil)
		defer resp.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, 200)
		So(string(b), ShouldEqual, "en")
	})

	Convey("it should return CORS headers when calling functions", t, func() {
		body := bytes.NewBuffer([]byte("language"))
		resp, err := http.Post("http://0.0.0.0:31415/fn/jsSampleZome/getProperty", "", body)
		So(err, ShouldBeNil)
		defer resp.Body.Close()
		_, corsPresent := resp.Header["Access-Control-Allow-Origin"]
		So(corsPresent, ShouldEqual, true)
	})

	Convey("it should return Holochain errors from call functions as 400", t, func() {
		body := bytes.NewBuffer([]byte("2"))
		resp, err := http.Post("http://0.0.0.0:31415/fn/jsSampleZome/addOdd", "", body)
		So(err, ShouldBeNil)
		defer resp.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, 400)
		So(string(b), ShouldEqual, `{"errorMessage":"Validation Failed: 2 is not odd","function":"commit","name":"HolochainError","source":{"column":"28","functionName":"addOdd","line":"45"}}
`)
	})

	Convey("it should return app thrown errors from call functions as 400", t, func() {
		body := bytes.NewBuffer([]byte("myError"))
		resp, err := http.Post("http://0.0.0.0:31415/fn/jsSampleZome/throwError", "", body)
		So(err, ShouldBeNil)
		defer resp.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, 400)
		So(string(b), ShouldEqual, "Error: myError\n")
	})

	fakeFromApp, _ := NewHash("QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx")
	token, _ := h.AddBridgeAsCallee(fakeFromApp, "")

	Convey("it should fail bridged functions without a good token", t, func() {
		body := bytes.NewBuffer([]byte("language"))
		resp, err := http.Post("http://0.0.0.0:31415/bridge/bogus_token/jsSampleZome/getProperty", "", body)
		So(err, ShouldBeNil)
		defer resp.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		So(string(b), ShouldEqual, "bridging error: invalid capability\n")
	})

	Convey("it should called bridged functions", t, func() {
		body := bytes.NewBuffer([]byte("language"))
		resp, err := http.Post("http://0.0.0.0:31415/bridge/"+token+"/jsSampleZome/getProperty", "", body)
		So(err, ShouldBeNil)
		defer resp.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		So(err, ShouldBeNil)
		So(string(b), ShouldEqual, "en")
	})
	ws.Stop()
	ws.Wait()
}
