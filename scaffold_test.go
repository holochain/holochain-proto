package holochain

import (
	"bytes"
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"strings"
	"testing"
)

func TestLoadDNAScaffold(t *testing.T) {
	scaffold := bytes.NewBuffer([]byte(BasicTemplateScaffold))
	Convey("we can load dna from a scaffold blob", t, func() {
		dna, err := LoadDNAScaffold(scaffold)
		So(err, ShouldBeNil)
		So(dna.Name, ShouldEqual, "templateApp")
		So(dna.Properties["description"], ShouldEqual, "provides an application template")
		So(fmt.Sprintf("%v", dna.UUID), ShouldEqual, "00000000-0000-0000-0000-000000000000")
		So(dna.Version, ShouldEqual, 1)
		So(dna.DHTConfig.HashType, ShouldEqual, "sha2-256")
		So(strings.Contains(dna.PropertiesSchema, `"properties"`), ShouldBeTrue)
		So(strings.Contains(dna.Zomes[0].Code, "function genesis"), ShouldBeTrue)
		So(dna.Zomes[0].Entries[0].Name, ShouldEqual, "testEntry")
		So(dna.Zomes[0].Functions[0].Name, ShouldEqual, "testEntryCreate")
	})
}
