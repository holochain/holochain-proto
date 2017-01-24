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
		So(IsInitialized(d), ShouldBeFalse)
	})

	agent := "Fred Flintstone <fred@flintstone.com>"

	s, err := Init(d, Agent(agent))
	Convey("when initializing service in a directory", t, func() {
		So(err, ShouldEqual, nil)

		Convey("it should return a service with default values", func() {
			So(s.DefaultAgent, ShouldEqual, Agent(agent))
			So(fmt.Sprintf("%v", s.Settings), ShouldEqual, "{6283 true false}")
		})

		p := d + "/" + DirectoryName
		Convey("it should create key files", func() {
			privP, err := UnmarshalPrivateKey(p, PrivKeyFileName)
			So(err, ShouldEqual, nil)

			pub2, err := UnmarshalPublicKey(p, PubKeyFileName)
			So(err, ShouldEqual, nil)

			So(fmt.Sprintf("%v", *pub2), ShouldEqual, fmt.Sprintf("%v", privP.PublicKey))
		})

		Convey("we can detect that it was initialized", func() {
			So(IsInitialized(d), ShouldBeTrue)
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
		So(s.Settings.Port, ShouldEqual, DefaultPort)
		So(s.DefaultAgent, ShouldEqual, Agent("Herbert <h@bert.com>"))
	})

}

func TestConfiguredChains(t *testing.T) {
	d, s, h := setupTestChain("test")
	defer cleanupTestDir(d)
	Convey("Configured chains should return a hash of all the chains in the Service", t, func() {
		chains, err := s.ConfiguredChains()
		So(err, ShouldBeNil)
		So(chains["test"].Id, ShouldEqual, h.Id)
	})
}
