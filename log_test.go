package holochain

import (
	"bytes"
	_ "github.com/op/go-logging"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

// needed to setup the holochain environment, not really a test.
func TestNewLog(t *testing.T) {
	var buf bytes.Buffer

	Convey("it should log according format string and prefix", t, func() {
		l1 := Logger{Enabled: true}
		err := l1.New(&buf)
		So(err, ShouldBeNil)

		l2 := Logger{
			Enabled: true,
			Format:  "L2:%{message}",
		}
		err = l2.New(&buf)
		So(err, ShouldBeNil)
		l1.Debug("fish")
		l2.Debugf("%d blue", 2)
		So(buf.String(), ShouldEqual, "fish\nL2:2 blue\n")
	})
}
