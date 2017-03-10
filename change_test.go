package holochain

import (
	"bytes"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

// needed to setup the holochain environment, not really a test.
func TestChange(t *testing.T) {
	c := Change{DeprecationMessage: "deprecated as of %s", AsOf: "0.0.2"}
	Convey("it should be able to log deprecation messages", t, func() {
		var buf bytes.Buffer
		w := log.w
		log.w = &buf
		e := log.Enabled
		log.Enabled = true
		c.Deprecated()
		So(buf.String(), ShouldEqual, "Deprecation warning: deprecated as of 0.0.2\n")
		log.Enabled = e
		log.w = w
	})
}
