package holochain

import (
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
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

func TestServiceGenChain(t *testing.T) {
	d, s, h := setupTestChain("test")
	defer cleanupTestDir(d)

	Convey("it should return a list of the chains", t, func() {
		list := s.ListChains()
		So(list, ShouldEqual, "installed holochains:     test <not-started>\n")
	})
	Convey("it should start a chain and return a holochain object", t, func() {
		h2, err := s.GenChain("test")
		So(err, ShouldBeNil)
		So(h2.nucleus.dna.UUID, ShouldEqual, h.nucleus.dna.UUID)
		list := s.ListChains()
		So(list, ShouldEqual, fmt.Sprintf("installed holochains:     test %v\n", h2.dnaHash))
	})
}

func TestCloneNew(t *testing.T) {
	d, s, h0 := setupTestChain("test")
	defer cleanupTestDir(d)

	name := "test2"
	root := s.Path + "/" + name

	orig := s.Path + "/test"
	Convey("it should create a chain from the examples directory", t, func() {
		h, err := s.Clone(orig, root, true)
		So(err, ShouldBeNil)
		So(h.nucleus.dna.Name, ShouldEqual, "test2")

		h, err = s.Load(name) // reload to confirm that it got saved correctly
		So(err, ShouldBeNil)

		So(h.nucleus.dna.Name, ShouldEqual, "test2")
		So(h.nucleus.dna.UUID, ShouldNotEqual, h0.nucleus.dna.UUID)

		agent, err := LoadAgent(s.Path)
		So(err, ShouldBeNil)
		So(h.agent.Name(), ShouldEqual, agent.Name())
		So(ic.KeyEqual(h.agent.PrivKey(), agent.PrivKey()), ShouldBeTrue)

		So(compareFile(orig+"/dna/zySampleZome", h.DNAPath()+"/zySampleZome", "zySampleZome.zy"), ShouldBeTrue)

		So(h.rootPath, ShouldEqual, root)
		So(h.UIPath(), ShouldEqual, root+"/ui")
		So(h.DNAPath(), ShouldEqual, root+"/dna")
		So(h.DBPath(), ShouldEqual, root+"/db")

		So(compareFile(orig+"/ui", h.UIPath(), "/index.html"), ShouldBeTrue)
		So(compareFile(orig+"/dna/zySampleZome", h.DNAPath()+"/zySampleZome", "profile.json"), ShouldBeTrue)
		So(compareFile(orig+"/dna", h.DNAPath(), "properties_schema.json"), ShouldBeTrue)
		So(compareFile(orig, h.rootPath, ConfigFileName+".toml"), ShouldBeTrue)

		So(compareFile(orig+"/"+ChainTestDir, h.rootPath+"/"+ChainTestDir, "test_0.json"), ShouldBeTrue)

		So(h.nucleus.dna.Progenitor.Name, ShouldEqual, "Herbert <h@bert.com>")
		pk, _ := agent.PubKey().Bytes()
		So(string(h.nucleus.dna.Progenitor.PubKey), ShouldEqual, string(pk))
	})
}

func TestCloneJoin(t *testing.T) {
	d, s, h0 := setupTestChain("test")
	defer cleanupTestDir(d)

	name := "test2"
	root := s.Path + "/" + name

	orig := s.Path + "/test"
	Convey("it should create a chain from the examples directory", t, func() {
		h, err := s.Clone(orig, root, false)
		So(err, ShouldBeNil)
		So(h.nucleus.dna.Name, ShouldEqual, "test")

		h, err = s.Load(name) // reload to confirm that it got saved correctly
		So(err, ShouldBeNil)

		So(h.nucleus.dna.Name, ShouldEqual, "test")
		So(h.nucleus.dna.UUID, ShouldEqual, h0.nucleus.dna.UUID)
		agent, err := LoadAgent(s.Path)
		So(err, ShouldBeNil)
		So(h.agent.Name(), ShouldEqual, agent.Name())
		So(ic.KeyEqual(h.agent.PrivKey(), agent.PrivKey()), ShouldBeTrue)
		src, _ := readFile(orig+"/dna/", "zySampleZome.zy")
		dst, _ := readFile(root, "zySampleZome.zy")
		So(string(src), ShouldEqual, string(dst))
		So(fileExists(h.UIPath()+"/index.html"), ShouldBeTrue)
		So(fileExists(h.DNAPath()+"/zySampleZome/profile.json"), ShouldBeTrue)
		So(fileExists(h.DNAPath()+"/properties_schema.json"), ShouldBeTrue)
		So(fileExists(h.rootPath+"/"+ConfigFileName+".toml"), ShouldBeTrue)

		So(h.nucleus.dna.Progenitor.Name, ShouldEqual, "Example Agent <example@example.com")
		pk := []byte{8, 1, 18, 32, 193, 43, 31, 148, 23, 249, 163, 154, 128, 25, 237, 167, 253, 63, 214, 220, 206, 131, 217, 74, 168, 30, 215, 237, 231, 160, 69, 89, 48, 17, 104, 210}
		So(string(h.nucleus.dna.Progenitor.PubKey), ShouldEqual, string(pk))

	})
}

func TestGenDev(t *testing.T) {
	d, s := setupTestService()
	defer cleanupTestDir(d)
	name := "test"
	root := s.Path + "/" + name

	Convey("we detected unconfigured holochains", t, func() {
		f, err := s.IsConfigured(name)
		So(f, ShouldEqual, "")
		So(err.Error(), ShouldEqual, "No DNA file in "+root+"/"+ChainDNADir+"/")
		_, err = s.load("test", "json")
		So(err.Error(), ShouldEqual, "open "+root+"/"+ChainDNADir+"/"+DNAFileName+".json: no such file or directory")

	})

	Convey("when generating a dev holochain", t, func() {
		h, err := s.GenDev(root, "json")
		So(err, ShouldBeNil)

		f, err := s.IsConfigured(name)
		So(err, ShouldBeNil)
		So(f, ShouldEqual, "json")

		h, err = s.Load(name)
		So(err, ShouldBeNil)

		lh, err := s.load(name, "json")
		So(err, ShouldBeNil)
		So(lh.nodeID, ShouldEqual, h.nodeID)
		So(lh.nodeIDStr, ShouldEqual, h.nodeIDStr)
		So(lh.config.Port, ShouldEqual, DefaultPort)
		So(h.config.PeerModeDHTNode, ShouldEqual, s.Settings.DefaultPeerModeDHTNode)
		So(h.config.PeerModeAuthor, ShouldEqual, s.Settings.DefaultPeerModeAuthor)
		So(h.config.BootstrapServer, ShouldEqual, s.Settings.DefaultBootstrapServer)

		So(fileExists(h.DNAPath()+"/zySampleZome/profile.json"), ShouldBeTrue)
		So(fileExists(h.UIPath()+"/index.html"), ShouldBeTrue)
		So(fileExists(h.UIPath()+"/hc.js"), ShouldBeTrue)
		So(fileExists(h.rootPath+"/"+ConfigFileName+".json"), ShouldBeTrue)

		Convey("we should not be able re generate it", func() {
			_, err = s.GenDev(root, "json")
			So(err.Error(), ShouldEqual, "holochain: "+root+" already exists")
		})
	})
}
