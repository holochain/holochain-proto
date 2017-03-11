package holochain

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

// needed to setup the holochain environment, not really a test.
func TestChange(t *testing.T) {
	Convey("it should be able to log deprecation messages", t, func() {
		c := Change{Type: Deprecation, Message: "deprecated as of %d", AsOf: 2}
		ShouldLog(&infoLog, "Deprecation warning: deprecated as of 2\n", func() {
			c.Log()
		})

	})

	Convey("it should be able to log requirement messages", t, func() {
		c := Change{Type: Warning, Message: "required as of %d", AsOf: 2}
		ShouldLog(&infoLog, "Warning: required as of 2\n", func() {
			c.Log()
		})
	})
}
