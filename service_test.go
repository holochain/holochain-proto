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

	s, err := Init(d+"/"+DefaultDirectoryName, AgentID(agent))
	Convey("when initializing service in a directory", t, func() {
		So(err, ShouldEqual, nil)

		Convey("it should return a service with default values", func() {
			So(s.DefaultAgent.ID(), ShouldEqual, AgentID(agent))
			So(fmt.Sprintf("%v", s.Settings), ShouldEqual, "{true true}")
		})

		p := d + "/" + DefaultDirectoryName
		Convey("it should create agent files", func() {
			a, err := LoadAgent(p)
			So(err, ShouldEqual, nil)
			So(a.ID(), ShouldEqual, AgentID(agent))
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
		So(s.DefaultAgent.ID(), ShouldEqual, AgentID("Herbert <h@bert.com>"))
	})

}

func TestConfiguredChains(t *testing.T) {
	d, s, h := setupTestChain("test")
	defer cleanupTestDir(d)
	// close the bolt instance so to call in ConfiguredChains doesn't timeout.
	h.store.Close()

	Convey("Configured chains should return a hash of all the chains in the Service", t, func() {
		chains, err := s.ConfiguredChains()
		So(err, ShouldBeNil)
		So(chains["test"].Id, ShouldEqual, h.Id)
	})
}
