package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
)

func TestNewNucleus(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
	var h Holochain
	h.rootPath = d
	os.MkdirAll(h.DBPath(), os.ModePerm)

	nucleus := NewNucleus(&h, &DNA{})
	Convey("It should initialize the Nucleus struct", t, func() {
		So(nucleus.h, ShouldEqual, &h)
		So(nucleus.alog, ShouldEqual, &h.config.Loggers.App)
	})
}

func TestAppMessages(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	// no need to activate DHT protocols for this test
	h.config.PeerModeDHTNode = false

	if err := h.Activate(); err != nil {
		panic(err)
	}
	Convey("it should fail on incorrect body types", t, func() {
		_, err := h.Send(ActionProtocol, h.node.HashAddr, APP_MESSAGE, GetReq{})
		So(err.Error(), ShouldEqual, "Unexpected request body type 'holochain.GetReq' in send request, expecting holochain.AppMsg")
	})

	Convey("it should fail on unknown zomes", t, func() {
		_, err := h.Send(ActionProtocol, h.node.HashAddr, APP_MESSAGE, AppMsg{ZomeType: "foo"})
		So(err.Error(), ShouldEqual, "unknown zome: foo")
	})

	Convey("it should send and receive app messages", t, func() {
		r, err := h.Send(ActionProtocol, h.node.HashAddr, APP_MESSAGE, AppMsg{ZomeType: "jsSampleZome", Body: `{"ping":"foobar"}`})
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", r), ShouldEqual, `{jsSampleZome {"pong":"foobar"}}`)
	})
}
