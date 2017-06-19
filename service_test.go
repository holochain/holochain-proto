package holochain

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestInit(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)

	Convey("we can detect an uninitialized directory", t, func() {
		So(IsInitialized(d+"/"+DefaultDirectoryName), ShouldBeFalse)
	})

	agent := "Fred Flintstone <fred@flintstone.com>"

	s, err := Init(d+"/"+DefaultDirectoryName, AgentName(agent))
	Convey("when initializing service in a directory", t, func() {
		So(err, ShouldEqual, nil)

		Convey("it should return a service with default values", func() {
			So(s.DefaultAgent.Name(), ShouldEqual, AgentName(agent))
			So(fmt.Sprintf("%v", s.Settings), ShouldEqual, "{true true bootstrap.holochain.net:10000}")
		})

		p := d + "/" + DefaultDirectoryName
		Convey("it should create agent files", func() {
			a, err := LoadAgent(p)
			So(err, ShouldEqual, nil)
			So(a.Name(), ShouldEqual, AgentName(agent))
		})

		Convey("we can detect that it was initialized", func() {
			So(IsInitialized(d+"/"+DefaultDirectoryName), ShouldBeTrue)
		})

		Convey("it should create an agent file", func() {
			a, err := readFile(p, AgentFileName)
			So(err, ShouldEqual, nil)
			So(string(a), ShouldEqual, agent)
		})
	})
}

func TestLoadService(t *testing.T) {
	d, service := setupTestService()
	root := service.Path
	defer cleanupTestDir(d)
	Convey("loading service from disk should set up the struct", t, func() {
		s, err := LoadService(root)
		So(err, ShouldEqual, nil)
		So(s.Path, ShouldEqual, root)
		So(s.Settings.DefaultPeerModeDHTNode, ShouldEqual, true)
		So(s.Settings.DefaultPeerModeAuthor, ShouldEqual, true)
		So(s.DefaultAgent.Name(), ShouldEqual, AgentName("Herbert <h@bert.com>"))
	})

}

func TestValidateServiceConfig(t *testing.T) {
	svc := ServiceConfig{}

	Convey("it should fail without one peer mode set to true", t, func() {
		err := svc.Validate()
		So(err.Error(), ShouldEqual, SysFileName+": At least one peer mode must be set to true.")
	})

	svc.DefaultPeerModeAuthor = true

	Convey("it should validate", t, func() {
		err := svc.Validate()
		So(err, ShouldBeNil)
	})

}

func TestConfiguredChains(t *testing.T) {
	d, s, h := setupTestChain("test")
	defer cleanupTestDir(d)

	Convey("Configured chains should return a hash of all the chains in the Service", t, func() {
		chains, err := s.ConfiguredChains()
		So(err, ShouldBeNil)
		So(chains["test"].nucleus.dna.UUID, ShouldEqual, h.nucleus.dna.UUID)
	})
}
