package main

import (
	"encoding/json"
	"fmt"
	holo "github.com/metacurrency/holochain"
	. "github.com/smartystreets/goconvey/convey"
	_ "github.com/urfave/cli"
	"testing"
	"time"
)

func TestSetupApp(t *testing.T) {
	app := setupApp()
	Convey("it should create the bootstrap server App", t, func() {
		So(app.Name, ShouldEqual, "bs")
	})
}

func TestPostGet(t *testing.T) {
	d := holo.SetupTestDir()
	defer holo.CleanupTestDir(d)

	Convey("it should setup the db", t, func() {
		err := setupDB(d + "bsdb.buntdb")
		So(err, ShouldBeNil)
	})

	halfOfBoostrapTTL := holo.BootstrapTTL - holo.BootstrapTTL/2

	Convey("it should store and retrieve and ignore old stuff", t, func() {
		chain := "QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXax"
		req1 := holo.BSReq{Version: 1, NodeID: "QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx", NodeAddr: "192.168.1.1"}
		now := time.Now()
		then := now.Add(-halfOfBoostrapTTL)

		err := post(chain, &req1, "172.3.4.1", then)
		So(err, ShouldBeNil)

		req2 := holo.BSReq{Version: 1, NodeID: "QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHy", NodeAddr: "192.168.1.2"}
		err = post(chain, &req2, "172.3.4.2", now)
		So(err, ShouldBeNil)

		wayback := now.Add(-holo.BootstrapTTL * 2)
		req3 := holo.BSReq{Version: 1, NodeID: "QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHz", NodeAddr: "192.168.1.3"}
		err = post(chain, &req3, "172.3.4.3", wayback)
		So(err, ShouldBeNil)

		result, err := get(chain)
		So(err, ShouldBeNil)
		So(result, ShouldEqual, fmt.Sprintf(`[{"Req":{"Version":1,"NodeID":"QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx","NodeAddr":"192.168.1.1"},"Remote":"172.3.4.1","LastSeen":%v},{"Req":{"Version":1,"NodeID":"QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHy","NodeAddr":"192.168.1.2"},"Remote":"172.3.4.2","LastSeen":%v}]`, jsonTime(then), jsonTime(now)))
	})

	Convey("it handle same node ID on different chains", t, func() {
		chain1 := "QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXa1"
		chain2 := "QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXa2"
		node := "QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx"
		now := time.Now()
		then := now.Add(-halfOfBoostrapTTL)
		req := holo.BSReq{Version: 1, NodeID: node, NodeAddr: "192.168.1.1"}
		err := post(chain1, &req, "172.3.4.1", now)
		So(err, ShouldBeNil)
		err = post(chain2, &req, "172.3.4.1", then)
		So(err, ShouldBeNil)

		result, err := get(chain1)
		So(err, ShouldBeNil)
		So(result, ShouldEqual, fmt.Sprintf(`[{"Req":{"Version":1,"NodeID":"QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx","NodeAddr":"192.168.1.1"},"Remote":"172.3.4.1","LastSeen":%v}]`, jsonTime(now)))

		result, err = get(chain2)
		So(err, ShouldBeNil)
		So(result, ShouldEqual, fmt.Sprintf(`[{"Req":{"Version":1,"NodeID":"QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHx","NodeAddr":"192.168.1.1"},"Remote":"172.3.4.1","LastSeen":%v}]`, jsonTime(then)))

	})
}

func jsonTime(t time.Time) string {
	b, _ := json.Marshal(t)
	return string(b)
}
