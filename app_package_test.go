package holochain

import (
	"bytes"
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"strings"
	"testing"
)

func TestLoadAppPackage(t *testing.T) {
	appPackageBlob := bytes.NewBuffer([]byte(BasicTemplateAppPackage))
	appPackage, err := LoadAppPackage(appPackageBlob, BasicTemplateAppPackageFormat)
	Convey("it should load dna from a appPackage blob", t, func() {
		So(err, ShouldBeNil)
		dna := appPackage.DNA
		So(dna.Name, ShouldEqual, "templateApp")
		So(dna.Properties["description"], ShouldEqual, "provides an application template")
		So(fmt.Sprintf("%v", dna.UUID), ShouldEqual, "00000000-0000-0000-0000-000000000000")
		So(dna.Version, ShouldEqual, 1)
		So(dna.DHTConfig.HashType, ShouldEqual, "sha2-256")
		So(strings.Contains(dna.PropertiesSchema, `"properties"`), ShouldBeTrue)
		So(strings.Contains(dna.Zomes[0].Code, "function genesis"), ShouldBeTrue)
		So(dna.Zomes[0].Entries[0].Name, ShouldEqual, "sampleEntry")
		So(dna.Zomes[0].Entries[0].Schema, ShouldEqual, "{\n	\"title\": \"sampleEntry Schema\",\n	\"type\": \"object\",\n	\"properties\": {\n		\"content\": {\n			\"type\": \"string\"\n		},\n		\"timestamp\": {\n			\"type\": \"integer\"\n		}\n	},\n    \"required\": [\"body\", \"timestamp\"]\n}")
		So(dna.Zomes[0].Functions[0].Name, ShouldEqual, "sampleEntryCreate")
	})

	Convey("it should load tests from a appPackage blob", t, func() {
		So(appPackage.TestSets[0].Name, ShouldEqual, "sample")
		So(appPackage.TestSets[0].TestSet.Tests[0].Convey, ShouldEqual, "We can create a new sampleEntry")
	})

	Convey("it should load scenarios from a appPackage blob", t, func() {
		So(appPackage.Scenarios[0].Name, ShouldEqual, "sampleScenario")
		So(appPackage.Scenarios[0].Roles[0].Name, ShouldEqual, "listener")
		So(len(appPackage.Scenarios[0].Roles[0].TestSet.Tests), ShouldEqual, 1)
		So(appPackage.Scenarios[0].Roles[0].TestSet.Tests[0].Convey, ShouldEqual, "add listener test here")
		So(appPackage.Scenarios[0].Roles[1].Name, ShouldEqual, "speaker")
		So(len(appPackage.Scenarios[0].Roles[1].TestSet.Tests), ShouldEqual, 1)
		So(appPackage.Scenarios[0].Roles[1].TestSet.Tests[0].Convey, ShouldEqual, "add speaker test here")
		So(appPackage.Scenarios[0].Config.Duration, ShouldEqual, 5)
		So(appPackage.Scenarios[0].Config.GossipInterval, ShouldEqual, 100)
	})
}
