package holochain

import (
	"bytes"
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

// needed to setup the holochain environment, not really a test.
func TestNewLog(t *testing.T) {

	Convey("it should log according format string", t, func() {
		var buf bytes.Buffer
		l1 := Logger{Enabled: true}
		err := l1.New(&buf)
		So(err, ShouldBeNil)

		l2 := Logger{
			Enabled: true,
			Format:  "L2:%{message}",
		}
		err = l2.New(&buf)
		So(err, ShouldBeNil)
		l1.Log("fish")
		l2.Logf("%d blue", 2)
		So(buf.String(), ShouldEqual, "fish\nL2:2 blue\n")
	})

	Convey("it should handle time", t, func() {
		var buf bytes.Buffer
		l := Logger{
			Enabled: true,
			Format:  "%{time}:%{message}",
		}

		tf, f := l.setupTime("X:%{message}")
		So(tf, ShouldEqual, "")
		So(f, ShouldEqual, "X:%{message}")

		tf, f = l.setupTime("x%{time}y")
		So(f, ShouldEqual, "x%{time}y")
		So(tf, ShouldEqual, "Jan _2 15:04:05")

		tf, f = l.setupTime("x%{time:xxy}y")
		So(f, ShouldEqual, "x%{time}y")
		So(tf, ShouldEqual, "xxy")

		l.New(&buf)
		now := time.Unix(1, 1)
		So(l._parse("fish", &now), ShouldEqual, now.Format(time.Stamp)+":fish")
	})

	Convey("it should handle color", t, func() {
		var buf bytes.Buffer
		l := Logger{
			Enabled: true,
			Format:  "%{color:blue}%{time}:%{message}",
		}

		c, f := l.setupColor("x")
		So(c, ShouldBeNil)
		So(f, ShouldEqual, "x")

		c, f = l.setupColor("prefix%{color:red}%{message}")
		So(fmt.Sprintf("%v", c), ShouldEqual, "&{[31] <nil>}")
		So(f, ShouldEqual, "prefix%{message}")

		l.New(&buf)
		now := time.Unix(1, 1)
		So(l._parse("fish", &now), ShouldEqual, now.Format(time.Stamp)+":fish")
	})

	Convey("it should log with a prefix", t, func() {
		var buf bytes.Buffer
		l := Logger{
			Enabled: true,
		}
		l.New(&buf)

		l.Log("onefish")
		l.SetPrefix("[PREFIX]")
		l.Log("twofish")
		l.SetPrefix("[COLOR PREFIX]")
		l.Log("threefish")
		So(buf.String(), ShouldEqual, "onefish\n[PREFIX]twofish\n[COLOR PREFIX]threefish\n")
	})

	Convey("it should handle file name and line number", t, func() {
		var buf bytes.Buffer
		l := Logger{
			Enabled: true,
			Format:  "%{file}.%{line}:%{message}",
		}
		l.New(&buf)
		doDebug(l, "fish")
		So(buf.String(), ShouldEqual, "log_test.go.97:fish\n")
	})

}

// we do this because in the case of file & line needs to know how many
// calls back up the stack to use for calculating the line number and we
// allways have a wrapper function around the log
func doDebug(l Logger, m string) {
	l.Log(m)
}
