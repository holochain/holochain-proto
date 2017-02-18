package holochain

import (
	_ "fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestPutGet(t *testing.T) {
	dht := NewDHT()
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
}
