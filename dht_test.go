package holochain

import (
	_ "fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

/*
func TestPutGet(t *testing.T) {
	dht := Needn't()
	Convey("It should store and retrieve", t, func() {
		h := NewHash("1vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")
		err := dht.Put(h)
		So(err, ShouldBeNil)
		data, err := dht.Get(h)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "fake value")
		data, err = dht.Get(NewHash("2vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA"))
		So(err.Error(), ShouldEqual, "No key: 2vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA")

	})
}*/

func TestNewDHT(t *testing.T) {
	var h Holochain

	dht := NewDHT(&h)
	Convey("It should initialize the DHT struct", t, func() {
		So(dht.h, ShouldEqual, &h)
	})
}

func TestFindNodeForHash(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)
	//	var err error

	Convey("It should find a node", t, func() {
		// for now the node it finds is ourself
		So(h.dht, ShouldNotBeNil)
	})
}
