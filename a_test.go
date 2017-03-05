package holochain

import (
	"github.com/op/go-logging"
	"os"
	"testing"
)

// needed to setup the holochain environment, not really a test.
func Test(t *testing.T) {
	l := logging.INFO
	if os.Getenv("DEBUG") == "1" {
		l = logging.DEBUG
	}

	log = logging.MustGetLogger("holochain")
	logging.SetLevel(l, "holochain")
	Register(log)
}
