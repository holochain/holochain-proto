// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Service implements functions and data that provide Holochain services in a unix file based environment

package holochain

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/google/uuid"
	. "github.com/metacurrency/holochain/hash"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// System settings, directory, and file names
const (
	DefaultDirectoryName string = ".holochain"  // Directory for storing config data
	ChainDataDir         string = "db"          // Sub-directory for all chain content files
	ChainDNADir          string = "dna"         // Sub-directory for all chain definition files
	ChainUIDir           string = "ui"          // Sub-directory for all chain user interface files
	ChainTestDir         string = "test"        // Sub-directory for all chain test files
	DNAFileName          string = "dna"         // Definition of the Holochain
	ConfigFileName       string = "config"      // Settings of the Holochain
	SysFileName          string = "system.conf" // Server & System settings
	AgentFileName        string = "agent.txt"   // User ID info
	PrivKeyFileName      string = "priv.key"    // Signing key - private
	StoreFileName        string = "chain.db"    // Filename for local data store
	DNAHashFileName      string = "dna.hash"    // Filename for storing the hash of the holochain
	DHTStoreFileName     string = "dht.db"      // Filname for storing the dht
	BridgeDBFileName     string = "bridge.db"   // Filname for storing bridge keys

	TestConfigFileName string = "_config.json"

	DefaultPort            = 6283
	DefaultBootstrapServer = "bootstrap.holochain.net:10000"

	DefaultBootstrapServerEnvVar = "HC_DEFAULT_BOOTSTRAPSERVER"
	DefaultEnableMDNSEnvVar      = "HC_DEFAULT_ENABLEMDNS"
	DefaultEnableNATUPnPEnvVar   = "HC_DEFAULT_ENABLENATUPNP"

	//HC_BOOTSTRAPPORT						= "HC_BOOTSTRAPPORT"

	CloneWithNewUUID  = true
	CloneWithSameUUID = false
	InitializeDB      = true
	SkipInitializeDB  = false
)

//
type CloneSpec struct {
	Role   string
	Number int
}

// TestConfig holds the configuration options for a test
type TestConfig struct {
	GossipInterval int // interval in milliseconds between gossips
	Duration       int // if non-zero number of seconds to keep all nodes alive
	Clone          []CloneSpec
}

// ServiceConfig holds the service settings
type ServiceConfig struct {
	DefaultPeerModeAuthor  bool
	DefaultPeerModeDHTNode bool
	DefaultBootstrapServer string
	DefaultEnableMDNS      bool
	DefaultEnableNATUPnP   bool
}

// A Service is a Holochain service data structure
type Service struct {
	Settings     ServiceConfig
	DefaultAgent Agent
	Path         string
}

type EntryDefFile struct {
	Name       string
	DataFormat string
	Schema     string
	SchemaFile string // file name of schema or language schema directive
	Sharing    string
}

type ZomeFile struct {
	Name         string
	Description  string
	CodeFile     string
	Entries      []EntryDefFile
	RibosomeType string
	Functions    []FunctionDef
	BridgeFuncs  []string // functions in zome that can be bridged to by fromApp
	BridgeTo     string   // dna Hash of toApp that this zome is a client of
}

type DNAFile struct {
	Version              int
	UUID                 uuid.UUID
	Name                 string
	Properties           map[string]string
	PropertiesSchemaFile string
	BasedOn              Hash // references hash of another holochain that these schemas and code are derived from
	Zomes                []ZomeFile
	RequiresVersion      int
	DHTConfig            DHTConfig
	Progenitor           Progenitor
}

// TestData holds a test entry for a chain
type TestData struct {
	Convey   string        // a human readable description of the tests intent
	Zome     string        // the zome in which to find the function
	FnName   string        // the function to call
	Input    interface{}   // the function's input
	Output   interface{}   // the expected output to match against (full match)
	Err      string        // the expected error to match against
	Regexp   string        // the expected out to match again (regular expression)
	Time     time.Duration // offset in milliseconds from the start of the test at which to run this test.
	Wait     time.Duration // time in milliseconds to wait before running this test from when the previous ran
	Exposure string        // the exposure context for the test call (defaults to ZOME_EXPOSURE)
	Raw      bool          // set to true if we should ignore fnName and just call input as raw code in the zome, useful for testing helper functions and validation functions
	Repeat   int           // number of times to repeat this test, useful for scenario testing
}

// IsDevMode is used to enable certain functionality when developing holochains, for example,
// in dev mode, you can put the name of an app in the BridgeTo of the DNA and it will get
// resolved to DNA hash of the app in the DevDNAResolveMap[name] global variable.
var IsDevMode bool = false
var DevDNAResolveMap map[string]string

// IsInitialized checks a path for a correctly set up .holochain directory
func IsInitialized(root string) bool {
	return DirExists(root) && FileExists(filepath.Join(root, SysFileName)) && FileExists(filepath.Join(root, AgentFileName))
}

// Init initializes service defaults including a signing key pair for an agent
// and writes them out to configuration files in the root path (making the
// directory if necessary)
func Init(root string, identity AgentIdentity, seed io.Reader) (service *Service, err error) {
	err = os.MkdirAll(root, os.ModePerm)
	if err != nil {
		return
	}
	s := Service{
		Settings: ServiceConfig{
			DefaultPeerModeDHTNode: true,
			DefaultPeerModeAuthor:  true,
			DefaultBootstrapServer: DefaultBootstrapServer,
			DefaultEnableMDNS:      false,
			DefaultEnableNATUPnP:   false,
		},
		Path: root,
	}

	if os.Getenv(DefaultBootstrapServerEnvVar) != "" {
		s.Settings.DefaultBootstrapServer = os.Getenv(DefaultBootstrapServerEnvVar)
		Infof("Using %s--configuring default bootstrap server as: %s\n", DefaultBootstrapServerEnvVar, s.Settings.DefaultBootstrapServer)
	}

	if os.Getenv(DefaultEnableMDNSEnvVar) != "" && os.Getenv(DefaultEnableMDNSEnvVar) != "false" {
		s.Settings.DefaultEnableMDNS = true
		Infof("Using %s--configuring default MDNS use as: %v.\n", DefaultEnableMDNSEnvVar, s.Settings.DefaultEnableMDNS)
	}

	if os.Getenv(DefaultEnableNATUPnPEnvVar) != "" && os.Getenv(DefaultEnableNATUPnPEnvVar) != "false" {
		s.Settings.DefaultEnableNATUPnP = true
		Infof("Using %s--configuring default MDNS use as: %v.\n", DefaultEnableNATUPnPEnvVar, s.Settings.DefaultEnableNATUPnP)
	}

	err = writeToml(root, SysFileName, s.Settings, false)
	if err != nil {
		return
	}

	a, err := NewAgent(LibP2P, identity, seed)
	if err != nil {
		return
	}
	err = SaveAgent(root, a)
	if err != nil {
		return
	}

	s.DefaultAgent = a

	service = &s
	return
}

// LoadService creates a service object from a configuration file
func LoadService(path string) (service *Service, err error) {
	agent, err := LoadAgent(path)
	if err != nil {
		return
	}
	s := Service{
		Path:         path,
		DefaultAgent: agent,
	}

	_, err = toml.DecodeFile(filepath.Join(path, SysFileName), &s.Settings)
	if err != nil {
		return
	}

	if err = s.Settings.Validate(); err != nil {
		return
	}

	service = &s
	return
}

// Validate validates settings values
func (c *ServiceConfig) Validate() (err error) {
	if !(c.DefaultPeerModeAuthor || c.DefaultPeerModeDHTNode) {
		err = errors.New(SysFileName + ": At least one peer mode must be set to true.")
		return
	}
	return
}

// ConfiguredChains returns a list of the configured chains for the given service
func (s *Service) ConfiguredChains() (chains map[string]*Holochain, err error) {
	files, err := ioutil.ReadDir(s.Path)
	if err != nil {
		return
	}
	chains = make(map[string]*Holochain)
	for _, f := range files {
		if f.IsDir() {
			h, err := s.Load(f.Name())
			if err == nil {
				chains[f.Name()] = h
			}
		}
	}
	return
}

// Find the DNA files
func findDNA(path string) (f string, err error) {
	p := filepath.Join(path, DNAFileName)

	matches, err := filepath.Glob(p + ".*")
	if err != nil {
		return
	}
	for _, fn := range matches {
		f = EncodingFormat(fn)
		if f != "" {
			break
		}
	}

	if f == "" {
		err = fmt.Errorf("No DNA file in %s/", path)
		return
	}
	return
}

// IsConfigured checks a directory for correctly set up holochain configuration file
func (s *Service) IsConfigured(name string) (f string, err error) {
	root := filepath.Join(s.Path, name)

	f, err = findDNA(filepath.Join(root, ChainDNADir))
	if err != nil {
		return
	}
	//@todo check other things?

	return
}

// Load instantiates a Holochain instance from disk
func (s *Service) Load(name string) (h *Holochain, err error) {
	f, err := s.IsConfigured(name)
	if err != nil {
		return
	}
	h, err = s.load(name, f)
	return
}

// loadDNA decodes a DNA from a directory hierarchy as specified by a DNAFile
func (s *Service) loadDNA(path string, filename string, format string) (dnaP *DNA, err error) {
	var dnaFile DNAFile
	var dna DNA
	dnafile := filepath.Join(path, filename+"."+format)
	f, err := os.Open(dnafile)
	if err != nil {
		err = fmt.Errorf("error opening DNA file %s: %v", dnafile, err)
		return
	}
	defer f.Close()

	err = Decode(f, format, &dnaFile)
	if err != nil {
		err = fmt.Errorf("error decoding DNA file %s: %v", dnafile, err)
		return
	}

	var validator SchemaValidator
	var propertiesSchema []byte
	if dnaFile.PropertiesSchemaFile != "" {
		propertiesSchema, err = ReadFile(path, dnaFile.PropertiesSchemaFile)
		if err != nil {
			err = fmt.Errorf("error reading properties Schema file %s: %v", dnaFile.PropertiesSchemaFile, err)
			return
		}
		schemapath := filepath.Join(path, dnaFile.PropertiesSchemaFile)
		validator, err = BuildJSONSchemaValidatorFromFile(schemapath)
		if err != nil {
			err = fmt.Errorf("error building validator for %s: %v", schemapath, err)
			return
		}
	}

	dna.Version = dnaFile.Version
	dna.UUID = dnaFile.UUID
	dna.Name = dnaFile.Name
	dna.BasedOn = dnaFile.BasedOn
	dna.RequiresVersion = dnaFile.RequiresVersion
	dna.DHTConfig = dnaFile.DHTConfig
	dna.Progenitor = dnaFile.Progenitor
	dna.Properties = dnaFile.Properties
	dna.PropertiesSchema = string(propertiesSchema)
	dna.propertiesSchemaValidator = validator

	err = dna.check()
	if err != nil {
		err = fmt.Errorf("dna failed check with: %v", err)
		return
	}

	dna.Zomes = make([]Zome, len(dnaFile.Zomes))
	for i, zome := range dnaFile.Zomes {
		if zome.CodeFile == "" {
			var ext string
			switch zome.RibosomeType {
			case "js":
				ext = ".js"
			case "zygo":
				ext = ".zy"
			}
			dnaFile.Zomes[i].CodeFile = zome.Name + ext
		}

		zomePath := filepath.Join(path, zome.Name)
		codeFilePath := filepath.Join(zomePath, zome.CodeFile)
		if !FileExists(codeFilePath) {
			return nil, errors.New("DNA specified code file missing: " + zome.CodeFile)
		}

		dna.Zomes[i].Name = zome.Name
		dna.Zomes[i].Description = zome.Description
		dna.Zomes[i].RibosomeType = zome.RibosomeType
		dna.Zomes[i].Functions = zome.Functions
		dna.Zomes[i].BridgeFuncs = zome.BridgeFuncs
		if zome.BridgeTo != "" {
			dna.Zomes[i].BridgeTo, err = NewHash(zome.BridgeTo)
			if err != nil {
				// if in dev mode assume the bridgeTo was the app name
				// and that hcdev put the actual DNA for us in the DevDNAResolveMap
				if IsDevMode {
					var dnaHashStr string
					if DevDNAResolveMap != nil {
						dnaHashStr, _ = DevDNAResolveMap[zome.BridgeTo]
					}

					dna.Zomes[i].BridgeTo, err = NewHash(dnaHashStr)
					if err != nil {
						// if that doesn't work, assume the testing is for
						// a non bridged case, and just clear the bridgeTo value
						// but issue a warning.
						Infof("DEV MODE: WARNING, found BridgeTo value '%s' but unable to resolve, proceeding without BridgeTo", zome.BridgeTo)
					} else {
						Infof("DEV MODE: Found BridgeTo value '%s' and resolved to DNA Hash: %s", zome.BridgeTo, dnaHashStr)
					}
				} else {
					err = fmt.Errorf("in zome: %s BridgeTo hash invalid", zome.Name)
					return
				}
			}
		}

		var code []byte
		code, err = ReadFile(zomePath, zome.CodeFile)
		if err != nil {
			return
		}
		dna.Zomes[i].Code = string(code[:])

		dna.Zomes[i].Entries = make([]EntryDef, len(zome.Entries))
		for j, entry := range zome.Entries {
			dna.Zomes[i].Entries[j].Name = entry.Name
			dna.Zomes[i].Entries[j].DataFormat = entry.DataFormat
			dna.Zomes[i].Entries[j].Sharing = entry.Sharing
			dna.Zomes[i].Entries[j].Schema = entry.Schema
			if entry.Schema == "" && entry.SchemaFile != "" {
				schemaFilePath := filepath.Join(zomePath, entry.SchemaFile)
				if !FileExists(schemaFilePath) {
					return nil, errors.New("DNA specified schema file missing: " + schemaFilePath)
				}
				var schema []byte
				schema, err = ReadFile(zomePath, entry.SchemaFile)
				if err != nil {
					return
				}
				dna.Zomes[i].Entries[j].Schema = string(schema)
				if strings.HasSuffix(entry.SchemaFile, ".json") {
					if err = dna.Zomes[i].Entries[j].BuildJSONSchemaValidator(schemaFilePath); err != nil {
						err = fmt.Errorf("error building validator for %s: %v", schemaFilePath, err)
						return nil, err
					}
				}
			}
		}
	}

	dnaP = &dna
	return
}

// load unmarshals a holochain structure for the named chain and format
func (s *Service) load(name string, format string) (hP *Holochain, err error) {
	var h Holochain
	root := filepath.Join(s.Path, name)

	// load the config
	var f *os.File
	f, err = os.Open(filepath.Join(root, ConfigFileName+"."+format))
	if err != nil {
		return
	}
	defer f.Close()
	err = Decode(f, format, &h.Config)
	if err != nil {
		return
	}
	if err = h.Config.Setup(); err != nil {
		return
	}

	dna, err := s.loadDNA(filepath.Join(root, ChainDNADir), DNAFileName, format)
	if err != nil {
		return
	}

	h.encodingFormat = format
	h.rootPath = root
	h.nucleus = NewNucleus(&h, dna)

	// try and get the holochain-specific agent info
	agent, err := LoadAgent(root)
	if err != nil {
		// if not specified for this app, get the default from the Agent.txt file for all apps
		agent, err = LoadAgent(filepath.Dir(root))
	}

	// TODO verify Agent identity against schema
	if err != nil {
		return
	}
	h.agent = agent

	// once the agent is set up we can calculate the id
	h.nodeID, h.nodeIDStr, err = agent.NodeID()
	if err != nil {
		return
	}

	if err = h.PrepareHashType(); err != nil {
		return
	}

	h.chain, err = NewChainFromFile(h.hashSpec, filepath.Join(h.DBPath(), StoreFileName))
	if err != nil {
		return
	}

	// if the chain has been started there should be a DNAHashFile which
	// we can load to check against the actual hash of the DNA entry
	if len(h.chain.Headers) > 0 {
		h.dnaHash = h.chain.Headers[0].EntryLink.Clone()

		var b []byte
		b, err = ReadFile(h.rootPath, DNAHashFileName)
		if err == nil {
			if h.dnaHash.String() != string(b) {
				err = errors.New("DNA doesn't match file!")
				return
			}
		}
	}

	// @TODO compare value from file to actual hash

	if h.chain.Length() > 0 {
		h.agentHash = h.chain.Headers[1].EntryLink
		_, topHeader := h.chain.TopType(AgentEntryType)
		h.agentTopHash = topHeader.EntryLink
	}
	hP = &h
	return
}

// gen calls a make function which should build the holochain structure and supporting files
func gen(root string, initDB bool, makeH func(root string) (hP *Holochain, err error)) (h *Holochain, err error) {
	if DirExists(root) {
		return nil, mkErr(root + " already exists")
	}
	if err := os.MkdirAll(root, os.ModePerm); err != nil {
		return nil, err
	}

	// cleanup the directory if we enounter an error while generating
	defer func() {
		if err != nil {
			os.RemoveAll(root)
		}
	}()

	h, err = makeH(root)
	if err != nil {
		return nil, err
	}

	if initDB {
		if err := os.MkdirAll(h.DBPath(), os.ModePerm); err != nil {
			return nil, err
		}

		h.chain, err = NewChainFromFile(h.hashSpec, filepath.Join(h.DBPath(), StoreFileName))
		if err != nil {
			return nil, err
		}
	}
	return
}

func suffixByRibosomeType(ribosomeType string) (suffix string) {
	switch ribosomeType {
	case JSRibosomeType:
		suffix = ".js"
	case ZygoRibosomeType:
		suffix = ".zy"
	default:
	}
	return
}

func _makeConfig(s *Service) (config Config, err error) {
	config = Config{
		Port:            DefaultPort,
		PeerModeDHTNode: s.Settings.DefaultPeerModeDHTNode,
		PeerModeAuthor:  s.Settings.DefaultPeerModeAuthor,
		BootstrapServer: s.Settings.DefaultBootstrapServer,
		EnableNATUPnP:   s.Settings.DefaultEnableNATUPnP,
		Loggers: Loggers{
			Debug:      Logger{Name: "Debug", Format: "HC: %{file}.%{line}: %{message}", Enabled: false},
			App:        Logger{Name: "App", Format: "%{color:cyan}%{message}", Enabled: false},
			DHT:        Logger{Name: "DHT", Format: "%{color:yellow}%{time} DHT: %{message}"},
			Gossip:     Logger{Name: "Gossip", Format: "%{color:blue}%{time} Gossip: %{message}"},
			TestPassed: Logger{Name: "TestPassed", Format: "%{color:green}%{message}", Enabled: true},
			TestFailed: Logger{Name: "TestFailed", Format: "%{color:red}%{message}", Enabled: true},
			TestInfo:   Logger{Name: "TestInfo", Format: "%{message}", Enabled: true},
		},
	}

	val := os.Getenv("HOLOCHAINCONFIG_PORT")
	if val != "" {
		Debugf("makeConfig: using environment variable to set port to: %s", val)
		config.Port, err = strconv.Atoi(val)
		if err != nil {
			return
		}
		Debugf("makeConfig: using environment variable to set port to: %v\n", val)
	}
	val = os.Getenv("HOLOCHAINCONFIG_BOOTSTRAP")
	if val != "" {
		if val == "_" {
			val = ""
		}
		config.BootstrapServer = val
		if val == "" {
			val = "NO BOOTSTRAP SERVER"
		}
		Debugf("makeConfig: using environment variable to set bootstrap server to: %s", val)
	}

	val = os.Getenv("HOLOCHAINCONFIG_ENABLEMDNS")
	if val != "" {
		Debugf("makeConfig: using environment variable to set enableMDNS to: %s", val)
		config.EnableMDNS = val == "true"
	}

	val = os.Getenv("HOLOCHAINCONFIG_ENABLENATUPNP")
	if val != "" {
		Debugf("makeConfig: using environment variable to set enableNATUPnP to: %s", val)
		config.EnableNATUPnP = val == "true"
	}
	return
}

func makeConfig(h *Holochain, s *Service) (err error) {
	h.Config, err = _makeConfig(s)
	if err != nil {
		return
	}
	p := filepath.Join(h.rootPath, ConfigFileName+"."+h.encodingFormat)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = Encode(f, h.encodingFormat, &h.Config); err != nil {
		return
	}
	if err = h.Config.Setup(); err != nil {
		return
	}
	return
}

func (service *Service) InitAppDir(root string, encodingFormat string) (err error) {
	var config Config
	config, err = _makeConfig(service)
	if err != nil {
		return
	}
	p := filepath.Join(root, ConfigFileName+"."+encodingFormat)
	var f, f1 *os.File
	f, err = os.Create(p)
	if err != nil {
		return
	}
	defer f.Close()

	if err = Encode(f, encodingFormat, &config); err != nil {
		return
	}

	if err = os.MkdirAll(filepath.Join(root, ChainDataDir), os.ModePerm); err != nil {
		return
	}

	f1, err = os.Create(filepath.Join(root, ChainDataDir, StoreFileName))
	if err != nil {
		return
	}
	defer f1.Close()
	return
}

// MakeTestingApp generates a holochain used for testing purposes
func (s *Service) MakeTestingApp(root string, encodingFormat string, initDB bool, newUUID bool, agent Agent) (h *Holochain, err error) {
	if DirExists(root) {
		return nil, mkErr(root + " already exists")
	}

	appPackageReader := bytes.NewBuffer([]byte(TestingAppAppPackage()))

	name := filepath.Base(root)

	_, err = s.SaveFromAppPackage(appPackageReader, root, "test", agent, encodingFormat, newUUID)
	if err != nil {
		return
	}
	if err = mkChainDirs(root, initDB); err != nil {
		return
	}

	if initDB {
		var config Config
		config, err = _makeConfig(s)
		if err != nil {
			return
		}
		p := filepath.Join(root, ConfigFileName+"."+encodingFormat)
		var f, f1 *os.File
		f, err = os.Create(p)
		if err != nil {
			return
		}
		defer f.Close()

		if err = Encode(f, encodingFormat, &config); err != nil {
			return
		}

		f1, err = os.Create(filepath.Join(root, ChainDataDir, StoreFileName))
		if err != nil {
			return
		}
		defer f1.Close()

		h, err = s.Load(name)
		if err != nil {
			return
		}
		if err = h.Config.Setup(); err != nil {
			return
		}

	}
	return
}

// if the directories don't exist, make the place to store chains
func mkChainDirs(root string, initDB bool) (err error) {
	if initDB {
		if err = os.MkdirAll(filepath.Join(root, ChainDataDir), os.ModePerm); err != nil {
			return err
		}
	}
	if err = os.MkdirAll(filepath.Join(root, ChainUIDir), os.ModePerm); err != nil {
		return
	}
	if err = os.MkdirAll(filepath.Join(root, ChainTestDir), os.ModePerm); err != nil {
		return
	}
	return
}

// Clone copies DNA files from a source directory
// bool new indicates if this clone should create a new DNA (when true) or act as a Join
func (s *Service) Clone(srcPath string, root string, agent Agent, new bool, initDB bool) (hP *Holochain, err error) {
	hP, err = gen(root, initDB, func(root string) (*Holochain, error) {
		var h Holochain
		srcDNAPath := filepath.Join(srcPath, ChainDNADir)

		format, err := findDNA(srcDNAPath)
		if err != nil {
			return nil, err
		}

		dna, err := s.loadDNA(srcDNAPath, DNAFileName, format)
		if err != nil {
			return nil, err
		}

		h.nucleus = NewNucleus(&h, dna)
		h.encodingFormat = format
		h.rootPath = root

		// create the DNA directory and copy
		if err := os.MkdirAll(h.DNAPath(), os.ModePerm); err != nil {
			return nil, err
		}

		// TODO verify identity against schema?
		h.agent = agent

		// once the agent is set up we can calculate the id
		h.nodeID, h.nodeIDStr, err = agent.NodeID()
		if err != nil {
			return nil, err
		}

		// make a config file
		if err = makeConfig(&h, s); err != nil {
			return nil, err
		}

		if new {
			h.nucleus.dna.NewUUID()

			// use the path as the name
			h.nucleus.dna.Name = filepath.Base(root)

			// change the progenitor to self because this is a clone!
			pk, err := agent.PubKey().Bytes()
			if err != nil {
				return nil, err
			}
			h.nucleus.dna.Progenitor = Progenitor{Identity: string(agent.Identity()), PubKey: pk}
		}

		// save out the DNA file
		if err = s.saveDNAFile(h.rootPath, h.nucleus.dna, h.encodingFormat, true); err != nil {
			return nil, err
		}

		// and the agent
		err = SaveAgent(h.rootPath, h.agent)
		if err != nil {
			return nil, err
		}

		// copy any UI files
		srcUIPath := filepath.Join(srcPath, ChainUIDir)
		if DirExists(srcUIPath) {
			if err = CopyDir(srcUIPath, h.UIPath()); err != nil {
				return nil, err
			}
		}

		// copy any test files
		srcTestDir := filepath.Join(srcPath, ChainTestDir)
		if DirExists(srcTestDir) {
			if err = CopyDir(srcTestDir, filepath.Join(root, ChainTestDir)); err != nil {
				return nil, err
			}
		}
		return &h, nil
	})
	return
}

func DNAHashofUngenedChain(h *Holochain) (DNAHash Hash, err error) {
	var buf bytes.Buffer

	err = h.EncodeDNA(&buf)
	e := GobEntry{C: buf.Bytes()}

	err = h.PrepareHashType()
	if err != nil {
		return
	}

	var dnaHeader *Header
	_, dnaHeader, err = newHeader(h.hashSpec, time.Now(), DNAEntryType, &e, h.agent.PrivKey(), NullHash(), NullHash(), nil)
	if err != nil {
		return
	}
	DNAHash = dnaHeader.EntryLink.Clone()
	return
}

// GenChain adds the genesis entries to a newly cloned or joined chain
func (s *Service) GenChain(name string) (h *Holochain, err error) {
	h, err = s.Load(name)
	if err != nil {
		return
	}
	_, err = h.GenChain()
	if err != nil {
		return
	}
	//	go h.DHT().HandleChangeReqs()
	return
}

// List chains produces a textual representation of the chains in the .holochain directory
func (s *Service) ListChains() (list string) {
	chains, _ := s.ConfiguredChains()
	l := len(chains)
	if l > 0 {
		keys := make([]string, l)
		i := 0
		for k := range chains {
			keys[i] = k
			i++
		}
		sort.Strings(keys)
		list = "installed holochains:\n"
		for _, k := range keys {
			id := chains[k].DNAHash()
			var sid = "<not-started>"
			if id.String() != "" {
				sid = id.String()
			}
			list += fmt.Sprintf("    %v %v\n", k, sid)
			bridges, _ := chains[k].GetBridges()
			if bridges != nil {
				for _, b := range bridges {
					if b.Side == BridgeFrom {
						list += fmt.Sprintf("        bridged to: %v\n", b.ToApp)
					} else {
						list += fmt.Sprintf("        bridged from by token: %v\n", b.Token)
					}
				}
			}
		}

	} else {
		list = "no installed chains"
	}
	return
}

// saveDNAFile writes out holochain DNA to files
func (s *Service) saveDNAFile(root string, dna *DNA, encodingFormat string, overwrite bool) (err error) {
	dnaPath := filepath.Join(root, ChainDNADir)
	p := filepath.Join(dnaPath, DNAFileName+"."+encodingFormat)
	if !overwrite && FileExists(p) {
		return mkErr(p + " already exists")
	}

	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	dnaFile := DNAFile{
		Version:              dna.Version,
		UUID:                 dna.UUID,
		Name:                 dna.Name,
		Properties:           dna.Properties,
		PropertiesSchemaFile: "properties_schema.json",
		BasedOn:              dna.BasedOn,
		RequiresVersion:      dna.RequiresVersion,
		DHTConfig:            dna.DHTConfig,
		Progenitor:           dna.Progenitor,
	}
	for _, z := range dna.Zomes {
		zpath := filepath.Join(dnaPath, z.Name)
		if err = os.MkdirAll(zpath, os.ModePerm); err != nil {
			return
		}
		if err = WriteFile([]byte(z.Code), zpath, z.Name+suffixByRibosomeType(z.RibosomeType)); err != nil {
			return
		}

		zomeFile := ZomeFile{Name: z.Name,
			Description:  z.Description,
			CodeFile:     z.CodeFileName(),
			RibosomeType: z.RibosomeType,
			Functions:    z.Functions,
			BridgeFuncs:  z.BridgeFuncs,
			BridgeTo:     z.BridgeTo.String(),
		}

		for _, e := range z.Entries {
			entryDefFile := EntryDefFile{
				Name:       e.Name,
				DataFormat: e.DataFormat,
				Sharing:    e.Sharing,
			}
			if e.DataFormat == DataFormatJSON && e.Schema != "" {
				entryDefFile.SchemaFile = e.Name + ".json"
				if err = WriteFile([]byte(e.Schema), zpath, e.Name+".json"); err != nil {
					return
				}
			}

			zomeFile.Entries = append(zomeFile.Entries, entryDefFile)
		}
		dnaFile.Zomes = append(dnaFile.Zomes, zomeFile)
	}

	if dna.PropertiesSchema != "" {
		if err = WriteFile([]byte(dna.PropertiesSchema), dnaPath, "properties_schema.json"); err != nil {
			return
		}
	}

	err = Encode(f, encodingFormat, dnaFile)
	return
}

// MakeAppPackage creates a package blob from a given holochain
func (service *Service) MakeAppPackage(h *Holochain) (data []byte, err error) {
	appPackage := AppPackage{
		Version:   AppPackageVersion,
		Generator: "holochain " + VersionStr,
		DNA:       *h.nucleus.dna,
	}

	var testsmap map[string][]TestData
	testsmap, err = LoadTestFiles(h.TestPath())
	if err != nil {
		return
	}
	appPackage.Tests = make([]AppPackageTests, 0)
	for name, t := range testsmap {
		appPackage.Tests = append(appPackage.Tests, AppPackageTests{Name: name, Tests: t})
	}

	var scenarioFiles map[string]*os.FileInfo
	scenarioFiles, err = GetTestScenarios(h)
	if err != nil {
		return
	}
	appPackage.Scenarios = make([]AppPackageScenario, 0)
	for name, _ := range scenarioFiles {
		scenarioPath := filepath.Join(h.TestPath(), name)
		var rolemap map[string][]TestData
		rolemap, err = LoadTestFiles(scenarioPath)
		if err != nil {
			return
		}
		roles := make([]AppPackageTests, 0)
		for name, tests := range rolemap {
			roles = append(roles,
				AppPackageTests{Name: name, Tests: tests})

		}
		scenario := AppPackageScenario{Name: name, Roles: roles}
		if FileExists(scenarioPath, TestConfigFileName) {
			var config *TestConfig
			config, err = LoadTestConfig(scenarioPath)
			if err != nil {
				return
			}
			scenario.Config = *config
		}
		appPackage.Scenarios = append(appPackage.Scenarios, scenario)
	}

	var files []os.FileInfo
	files, err = ioutil.ReadDir(h.UIPath())
	if err != nil {
		return
	}

	appPackage.UI = make([]AppPackageUIFile, 0)
	for _, f := range files {
		// TODO handle subdirectories
		if f.Mode().IsRegular() {
			var file []byte
			file, err = ReadFile(h.UIPath(), f.Name())
			if err != nil {
				return
			}
			uiFile := AppPackageUIFile{FileName: f.Name()}
			contentType := http.DetectContentType(file)
			if encodeAsBinary(contentType) {
				uiFile.Data = base64.StdEncoding.EncodeToString([]byte(file))

				uiFile.Encoding = "base64"
			} else {
				uiFile.Data = string(file)
			}
			appPackage.UI = append(appPackage.UI, uiFile)

		}
	}

	data, err = json.MarshalIndent(appPackage, "", "  ")
	return
}

func encodeAsBinary(contentType string) bool {
	if strings.HasPrefix(contentType, "text") {
		return false
	}
	return true
}

// SaveFromAppPackage writes out a holochain application based on appPackage file to path
func (service *Service) SaveFromAppPackage(reader io.Reader, path string, name string, agent Agent, encodingFormat string, newUUID bool) (appPackage *AppPackage, err error) {
	appPackage, err = LoadAppPackage(reader)
	if err != nil {
		return
	}
	err = service.saveFromAppPackage(appPackage, path, name, encodingFormat, newUUID)
	if err != nil {
		return
	}
	if agent == nil {
		agent = service.DefaultAgent
	}
	err = SaveAgent(path, agent)
	if err != nil {
		return
	}
	return
}

func (service *Service) saveFromAppPackage(appPackage *AppPackage, path string, name string, encodingFormat string, newUUID bool) (err error) {

	dna := &appPackage.DNA
	err = MakeDirs(path)
	if err != nil {
		return
	}
	if newUUID {
		dna.NewUUID()
	}
	dna.Name = name

	err = service.saveDNAFile(path, dna, encodingFormat, false)
	if err != nil {
		return
	}

	testPath := filepath.Join(path, ChainTestDir)
	for _, test := range appPackage.Tests {
		p := filepath.Join(testPath, test.Name+".json")
		var f *os.File
		f, err = os.Create(p)
		if err != nil {
			return
		}
		defer f.Close()
		err = Encode(f, "json", test.Tests)
		if err != nil {
			return
		}
	}

	for _, scenario := range appPackage.Scenarios {
		scenarioPath := filepath.Join(testPath, scenario.Name)
		err = os.MkdirAll(scenarioPath, os.ModePerm)
		if err != nil {
			return
		}
		for _, role := range scenario.Roles {
			p := filepath.Join(scenarioPath, role.Name+".json")
			var f *os.File
			f, err = os.Create(p)
			if err != nil {
				return
			}
			defer f.Close()
			err = Encode(f, "json", &role.Tests)
			if err != nil {
				return
			}
		}
		if scenario.Config.Duration != 0 {
			p := filepath.Join(scenarioPath, TestConfigFileName)
			var f *os.File
			f, err = os.Create(p)
			if err != nil {
				return
			}
			defer f.Close()
			err = Encode(f, "json", &scenario.Config)
		}
	}

	p := filepath.Join(path, ChainUIDir)
	for _, ui := range appPackage.UI {
		var data []byte
		if ui.Encoding == "base64" {
			data, err = base64.StdEncoding.DecodeString(ui.Data)
			if err != nil {
				return
			}
		} else {
			data = []byte(ui.Data)
		}

		if err = WriteFile(data, p, ui.FileName); err != nil {
			return
		}
	}

	return
}

//MakeDirs creates the directory structure of an application
func MakeDirs(devPath string) error {
	err := os.MkdirAll(devPath, os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(devPath, ChainDNADir), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(devPath, ChainUIDir), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(devPath, ChainTestDir), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

// LoadTestFile unmarshals test json data
func LoadTestFile(dir string, file string) (tests []TestData, err error) {
	var v []byte
	v, err = ReadFile(dir, file)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(v, &tests)

	if err != nil {
		return nil, err
	}
	return
}

// LoadTestConfig unmarshals test json data
func LoadTestConfig(dir string) (config *TestConfig, err error) {
	c := TestConfig{GossipInterval: 2000, Duration: 0}
	config = &c
	// if no config file return default values
	if !FileExists(dir, TestConfigFileName) {
		return
	}
	var v []byte
	v, err = ReadFile(dir, TestConfigFileName)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(v, &c)

	if err != nil {
		return nil, err
	}
	return
}

// LoadTestFiles searches a path for .json test files and loads them into an array
func LoadTestFiles(path string) (map[string][]TestData, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(.*)\.json`)
	var tests = make(map[string][]TestData)
	for _, f := range files {
		if f.Mode().IsRegular() {
			x := re.FindStringSubmatch(f.Name())
			if len(x) > 0 {
				if f.Name() != TestConfigFileName {
					name := x[1]
					tests[name], err = LoadTestFile(path, x[0])
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	if len(tests) == 0 {
		return nil, errors.New("no test files found in: " + path)
	}

	return tests, err
}

// TestScenarioList returns a list of paths to scenario directories
func GetTestScenarios(h *Holochain) (scenarios map[string]*os.FileInfo, err error) {
	dirContentList := []os.FileInfo{}
	scenarios = make(map[string]*os.FileInfo)

	dirContentList, err = ioutil.ReadDir(h.TestPath())
	if err != nil {
		return scenarios, err
	}
	for _, fileOrDir := range dirContentList {
		if fileOrDir.Mode().IsDir() {
			scenarios[fileOrDir.Name()] = &fileOrDir
		}
	}

	return scenarios, err
}

// GetScenarioDataMap returns a map of TestData object
func GetTestScenarioRoles(h *Holochain, scenarioName string) (roleNameList []string, err error) {
	return GetAllTestRoles(filepath.Join(h.TestPath(), scenarioName))
}

// GetAllTestRoles  retuns a list of the roles in a scenario
func GetAllTestRoles(path string) (roleNameList []string, err error) {
	roleNameList = []string{}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(.*)\.json`)
	for _, f := range files {
		if f.Mode().IsRegular() {
			x := re.FindStringSubmatch(f.Name())
			if len(x) > 0 {
				if f.Name() != TestConfigFileName {
					roleNameList = append(roleNameList, x[1])
				}
			}
		}
	}
	return
}

func TestingAppAppPackage() string {
	return `{
"Version": "` + AppPackageVersion + `",
"Generator": "holochain service.go",
"DNA": {
  "Version": 1,
  "UUID": "00000000-0000-0000-0000-000000000000",
  "Name": "testingApp",
  "RequiresVersion": ` + VersionStr + `,
  "Properties": {
    "description": "a bogus test holochain",
    "language": "en"
  },
  "PropertiesSchemaFile": "properties_schema.json",
  "DHTConfig": {
    "HashType": "sha2-256"
  },
  "Progenitor": {
      "Identity": "Progenitor Agent <progenitore@example.com>",
      "PubKey": [8, 1, 18, 32, 193, 43, 31, 148, 23, 249, 163, 154, 128, 25, 237, 167, 253, 63, 214, 220, 206, 131, 217, 74, 168, 30, 215, 237, 231, 160, 69, 89, 48, 17, 104, 210]
  },
  "Zomes": [
    {
      "Name": "zySampleZome",
      "Description": "this is a zygomas test zome",
      "RibosomeType": "zygo",
      "BridgeFuncs" :["testStrFn1"],
            "Entries": [
                {
                    "Name": "evenNumbers",
                    "DataFormat": "zygo",
                    "Sharing": "public"
                },
                {
                    "Name": "primes",
                    "DataFormat": "json",
                    "Schema": "` + jsSanitizeString(primesSchema) + `",
                    "Sharing": "public"
                },
                {
                    "Name": "profile",
                    "DataFormat": "json",
                    "Schema": "` + jsSanitizeString(profileSchema) + `",
                    "Sharing": "public"
                },
                {
                  "Name": "privateData",
                  "DataFormat": "string",
                  "Sharing": "private"
                }
            ],
            "Functions": [
                {
                    "Name": "getDNA",
                    "CallingType": "string",
                    "Exposure": ""
                },
                {
                    "Name": "addEven",
                    "CallingType": "string",
                    "Exposure": "public"
                },
                {
                    "Name": "addPrime",
                    "CallingType": "json",
                    "Exposure": "public"
                },
                {
                    "Name": "confirmOdd",
                    "CallingType": "string",
                    "Exposure": "public"
                },
                {
                    "Name": "testStrFn1",
                    "CallingType": "string",
                    "Exposure": ""
                },
                {
                    "Name": "testStrFn2",
                    "CallingType": "string",
                    "Exposure": ""
                },
                {
                    "Name": "testJsonFn1",
                    "CallingType": "json",
                    "Exposure": ""
                },
                {
                    "Name": "testJsonFn2",
                    "CallingType": "json",
                    "Exposure": ""
                }
            ],
      "Code": "` + jsSanitizeString(zygoZomeCode) + `"
    },
    {
      "Name": "jsSampleZome",
      "Description": "this is a javascript test zome",
      "RibosomeType": "js",
      "BridgeFuncs" :["getProperty"],
            "Entries": [
                {
                    "Name": "oddNumbers",
                    "DataFormat": "js",
                    "Sharing": "public"
                },
                {
                    "Name": "profile",
                    "DataFormat": "json",
                    "Schema": "` + jsSanitizeString(profileSchema) + `",
                    "Sharing": "public"
                },
                {
                    "Name": "rating",
                    "DataFormat": "links",
                },
                {
                    "Name": "review",
                    "DataFormat": "string",
                    "Sharing": "public"
                },
                {
                    "Name": "secret",
                    "DataFormat": "string",
                }
            ],
            "Functions": [
                {
                    "Name": "getProperty",
                    "CallingType": "string",
                    "Exposure": "public"
                },
                {
                    "Name": "addOdd",
                    "CallingType": "string",
                    "Exposure": "public"
                },
                {
                    "Name": "addProfile",
                    "CallingType": "json",
                    "Exposure": "public"
                },
                {
                    "Name": "testStrFn1",
                    "CallingType": "string",
                    "Exposure": ""
                },
                {
                    "Name": "testStrFn2",
                    "CallingType": "string",
                    "Exposure": ""
                },
                {
                    "Name": "testJsonFn1",
                    "CallingType": "json",
                    "Exposure": ""
                },
                {
                    "Name": "testJsonFn2",
                    "CallingType": "json",
                    "Exposure": ""
                }
            ],
      "Code": "` + jsSanitizeString(jsZomeCode) + `"
    }
  ]},
"Tests":[{"Name":"testSet1","Tests":
[
    {
        "Zome":   "zySampleZome",
        "FnName": "addEven",
        "Input":  "2",
        "Output": "%h%"},
    {
        "Zome":   "zySampleZome",
        "FnName": "addEven",
        "Input":  "4",
        "Output": "%h%"},
    {
        "Zome":   "zySampleZome",
        "FnName": "addEven",
        "Input":  "5",
        "Err":    "Error calling 'commit': Validation Failed"},
    {
        "Zome":   "zySampleZome",
        "FnName": "addPrime",
        "Input":  {"prime":7},
        "Output": "%h%"},
    {
        "Zome":   "zySampleZome",
        "FnName": "addPrime",
        "Input":  {"prime":4},
        "Err":    "Error calling 'commit': Validation Failed"},
    {
	"Zome":   "jsSampleZome",
	"FnName": "addProfile",
	"Input":  {"firstName":"Art","lastName":"Brock"},
	"Output": "%h%"},
    {
	"Zome":   "zySampleZome",
	"FnName": "getDNA",
	"Input":  "",
	"Output": "%dna%"},
    {
	"Zome":     "zySampleZome",
	"FnName":   "getDNA",
	"Input":    "",
	"Err":      "function not available",
	"Exposure":  "public"
    }
]
},{"Name":"testSet2","Tests":
[
    {
	"Zome":   "jsSampleZome",
	"FnName": "addOdd",
	"Input":  "7",
	"Output": "%h%"},
    {
	"Zome":   "jsSampleZome",
	"FnName": "addOdd",
	"Input":  "2",
	"Err":    "Validation Failed"},
    {
	"Zome":   "zySampleZome",
	"FnName": "confirmOdd",
	"Input":  "9",
	"Output": "false"},
    {
	"Zome":   "zySampleZome",
	"FnName": "confirmOdd",
	"Input":  "7",
	"Output": "true"},
    {
	"Zome":   "jsSampleZome",
	"Input":  "unexposed(\"this is a\")",
	"Output": "this is a fish",
	"Raw":    true
    },
    {
	"Convey": "test the output of a function that returns json",
	"Zome":   "jsSampleZome",
	"FnName": "testJsonFn2",
	"Input": "",
	"Output": ["a":"b"]
    }
]}],
"UI":[
{"FileName":"index.html",
 "Data":"` + jsSanitizeString(SampleHTML) + `"
},
{"FileName":"hc.js",
 "Data":"` + jsSanitizeString(SampleJS) + `"
},
{"FileName":"logo.png",
 "Data":"` + jsSanitizeString(SampleBinary) + `",
 "Encoding":"base64"
}],
"Scenarios":[
        {"Name":"sampleScenario",
         "Roles":[
             {"Name":"speaker",
              "Tests":[
                  {"Convey":"add an odd",
                   "Zome":   "jsSampleZome",
	           "FnName": "addOdd",
	           "Input":  "7",
	           "Output": "%h%"
                  }
               ]},
             {"Name":"listener",
              "Tests":[
                  {"Convey":"confirm prime exists",
                   "Zome":   "zySampleZome",
	           "FnName": "confirmOdd",
	           "Input":  "7",
	           "Output": "true",
                   "Time" : 1500
                  }
               ]},
          ],
         "Config":{"Duration":5,"GossipInterval":300}}]
}
`
}

const (
	profileSchema = `{
	"title": "Profile Schema",
	"type": "object",
	"properties": {
		"firstName": {
			"type": "string"
		},
		"lastName": {
			"type": "string"
		},
		"age": {
			"description": "Age in years",
			"type": "integer",
			"minimum": 0
		}
	},
	"required": ["firstName", "lastName"]
}`

	primesSchema = `
{
	"title": "Prime Schema",
	"type": "object",
	"properties": {
		"prime": {
			"type": "integer"
		}
	},
	"required": ["prime"]
}`

	jsZomeCode = `
function unexposed(x) {return x+" fish";};
function testStrFn1(x) {return "result: "+x};
function testStrFn2(x){ return parseInt(x)+2};
function testJsonFn1(x){ x.output = x.input*2; return x;};
function testJsonFn2(x){ return [{a:'b'}] };

function getProperty(x) {return property(x)};
function addOdd(x) {return commit("oddNumbers",x);}
function addProfile(x) {return commit("profile",x);}
function validatePut(entry_type,entry,header,pkg,sources) {
  return validate(entry_type,entry,header,sources);
}
function validateMod(entry_type,entry,header,replaces,pkg,sources) {
  return true;
}
function validateDel(entry_type,hash,pkg,sources) {
  return true;
}
function validateCommit(entry_type,entry,header,pkg,sources) {
  if (entry_type == "rating") {return true}
  return validate(entry_type,entry,header,sources);
}
function validate(entry_type,entry,header,sources) {
  if (entry_type=="oddNumbers") {
    return entry%2 != 0
  }
  if (entry_type=="profile") {
    return true
  }
  if (entry_type=="review") {
    return true
  }
  if (entry_type=="secret") {
    return true
  }
  return false
}
function validateLink(linkEntryType,baseHash,linkHash,tag,pkg,sources){return true}
function validatePutPkg(entry_type) {
  req = {};
  req[HC.PkgReq.Chain]=HC.PkgReq.ChainOpt.Full;
  return req;
}
function validateModPkg(entry_type) { return null}
function validateDelPkg(entry_type) { return null}
function validateLinkPkg(entry_type) { return null}

function genesis() {
  debug("running jsZome genesis")
  return true
}
function bridgeGenesis(side,app,data) {return true}

function receive(from,message) {
  // if the message requests blocking run an infinite loop
  // this is used by the async send test to force the condition where
  // the receiver doesn't return soon enough so that the send will timeout
  if (message.block) {
    while(true){};
  }

  // send back a pong message of what came in the ping message!
  return {pong:message.ping}
}

function testGetBridges() {
  debug(JSON.stringify(getBridges()))
}

function asyncPing(message,id) {
  debug("async result of message with "+id+" was: "+JSON.stringify(message))
}
`
	zygoZomeCode = `
(defn testStrFn1 [x] (concat "result: " x))
(defn testStrFn2 [x] (+ (atoi x) 2))
(defn testJsonFn1 [x] (begin (hset x output: (* (-> x input:) 2)) x))
(defn testJsonFn2 [x] (unjson (raw "[{\"a\":\"b\"}]"))) (defn getDNA [x] App_DNA_Hash)
(defn addEven [x] (commit "evenNumbers" x))
(defn addPrime [x] (commit "primes" x))
(defn confirmOdd [x]
  (letseq [h (makeHash "oddNumbers" x)
           r (get h)
           err (hget r %error "")]
     (cond (== err "") "true" "false")
  )
)
(defn validateCommit [entryType entry header pkg sources]
  (validate entryType entry header sources))
(defn validatePut [entryType entry header pkg sources]
  (validate entryType entry header sources))
(defn validateMod [entryType entry header replaces pkg sources] true)
(defn validateDel [entryType hash pkg sources] true)
(defn validate [entryType entry header sources]
  (cond (== entryType "evenNumbers")  (cond (== (mod entry 2) 0) true false)
        (== entryType "primes")  (isprime (hget entry %prime))
        (== entryType "profile") true
        false)
)
(defn validateLink [linkEntryType baseHash links pkg sources] true)
(defn validatePutPkg [entryType] nil)
(defn validateModPkg [entryType] nil)
(defn validateDelPkg [entryType] nil)
(defn validateLinkPkg [entryType] nil)
(defn genesis []
  (debug "running zyZome genesis")
  true
)
(defn bridgeGenesis [side app data] (begin (debug (concat "bridge genesis " (cond (== side HC_Bridge_From) "from" "to") "-- other side is:" app " bridging data:" data))  true))
(defn receive [from message]
	(hash pong: (hget message %ping)))

(defn testGetBridges []
  (debug (str (getBridges))))

(defn asyncPing [message,id]
  (debug (concat "async result of message with " id " was:" (str message)))
)
`

	SampleHTML = `
<html>
  <head>
    <title>Test</title>
    <script type="text/javascript" src="http://code.jquery.com/jquery-latest.js"></script>
    <script type="text/javascript" src="/hc.js">
    </script>
  </head>
  <body>
    <img src="logo.png">
    <select id="zome" name="zome">
      <option value="zySampleZome">zySampleZome</option>
      <option value="jsSampleZome">jsSampleZome</option>
    </select>
    <select id="fn" name="fn">
      <option value="addEven">addEven</option>
      <option value="getProperty">getProperty</option>
      <option value="addPrime">addPrime</option>
    </select>
    <input id="data" name="data">
    <button onclick="send();">Send</button>
    send an even number and get back a hash, send and odd end get a error

    <div id="result"></div>
    <div id="err"></div>
  </body>
</html>`
	SampleJS = `
     function send() {
         $.post(
             "/fn/"+$('select[name=zome]').val()+"/"+$('select[name=fn]').val(),
             $('#data').val(),
             function(data) {
                 $("#result").html("result:"+data)
                 $("#err").html("")
             }
         ).error(function(response) {
             $("#err").html(response.responseText)
             $("#result").html("")
         })
         ;
     };
`
	SampleBinary = `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
AABJRU5ErkJggg==
`
)
