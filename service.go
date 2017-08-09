// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// Service implements functions and data that provide Holochain services in a unix file based environment

package holochain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/google/uuid"

	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

	DefaultPort            = 6283
	DefaultBootstrapServer = "bootstrap.holochain.net:10000"

	HC_BOOTSTRAPSERVER = "HC_BOOTSTRAPSERVER"
	HC_ENABLEMDNS      = "HC_DEFAULT_ENABLEMDNS"
)

// ServiceConfig holds the service settings
type ServiceConfig struct {
	DefaultPeerModeAuthor  bool
	DefaultPeerModeDHTNode bool
	DefaultBootstrapServer string
	DefaultEnableMDNS      bool
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
	BridgeTo     Hash     // dna Hash of toApp for zomes to be included in the fromApp
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

// IsInitialized checks a path for a correctly set up .holochain directory
func IsInitialized(root string) bool {
	return dirExists(root) && fileExists(filepath.Join(root, SysFileName)) && fileExists(filepath.Join(root, AgentFileName))
}

// Init initializes service defaults including a signing key pair for an agent
// and writes them out to configuration files in the root path (making the
// directory if necessary)
func Init(root string, agent AgentIdentity) (service *Service, err error) {
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
		},
		Path: root,
	}

	if os.Getenv(HC_BOOTSTRAPSERVER) != "" {
		s.Settings.DefaultBootstrapServer = os.Getenv(HC_BOOTSTRAPSERVER)
		Infof("Using %s--configuring default bootstrap server as: %s\n", HC_BOOTSTRAPSERVER, s.Settings.DefaultBootstrapServer)
	}

	if os.Getenv(HC_ENABLEMDNS) != "" && os.Getenv(HC_ENABLEMDNS) != "false" {
		s.Settings.DefaultEnableMDNS = true
		Infof("Using %s--configuring default MDNS use as: %v.\n", HC_ENABLEMDNS, s.Settings.DefaultEnableMDNS)
	}

	err = writeToml(root, SysFileName, s.Settings, false)
	if err != nil {
		return
	}

	a, err := NewAgent(LibP2P, agent)
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
		s := strings.Split(fn, ".")
		f = s[len(s)-1]
		if f == "json" || f == "yml" || f == "yaml" || f == "toml" {
			break
		}
		f = ""
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

// LoadDNA decodes a DNA from a directory hierarchy as specified by a DNAFile
func (s *Service) LoadDNA(path string, filename string, format string) (dnaP *DNA, err error) {
	var dnaFile DNAFile
	var dna DNA
	dnafile := filepath.Join(path, filename+"."+format)
	//fmt.Printf("LoadDNA: opening dna file %s\n", filepath)
	f, err := os.Open(dnafile)
	if err != nil {
		return
	}
	defer f.Close()

	err = Decode(f, format, &dnaFile)
	if err != nil {
		return
	}

	var validator SchemaValidator
	var propertiesSchema []byte
	if dnaFile.PropertiesSchemaFile != "" {
		propertiesSchema, err = readFile(path, dnaFile.PropertiesSchemaFile)
		if err != nil {
			return
		}
		schemapath := filepath.Join(path, dnaFile.PropertiesSchemaFile)
		//fmt.Printf("LoadDNA: opening schema file %s\n", schemapath)
		validator, err = BuildJSONSchemaValidatorFromFile(schemapath)
		if err != nil {
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
		if !fileExists(codeFilePath) {
			//fmt.Printf("%v", zome)
			return nil, errors.New("DNA specified code file missing: " + zome.CodeFile)
		}

		dna.Zomes[i].Name = zome.Name
		dna.Zomes[i].Description = zome.Description
		dna.Zomes[i].RibosomeType = zome.RibosomeType
		dna.Zomes[i].Functions = zome.Functions
		dna.Zomes[i].BridgeFuncs = zome.BridgeFuncs
		dna.Zomes[i].BridgeTo = zome.BridgeTo

		var code []byte
		code, err = readFile(zomePath, zome.CodeFile)
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
				if !fileExists(schemaFilePath) {
					return nil, errors.New("DNA specified schema file missing: " + schemaFilePath)
				}
				var schema []byte
				schema, err = readFile(zomePath, entry.SchemaFile)
				if err != nil {
					return
				}
				dna.Zomes[i].Entries[j].Schema = string(schema)
				if strings.HasSuffix(entry.SchemaFile, ".json") {
					if err = dna.Zomes[i].Entries[j].BuildJSONSchemaValidator(schemaFilePath); err != nil {
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
	dna, err := s.LoadDNA(filepath.Join(root, ChainDNADir), DNAFileName, format)
	if err != nil {
		return
	}

	h.encodingFormat = format
	h.rootPath = root
	h.nucleus = NewNucleus(&h, dna)

	// load the config
	var f *os.File
	f, err = os.Open(filepath.Join(root, ConfigFileName+"."+format))
	if err != nil {
		return
	}
	defer f.Close()
	err = Decode(f, format, &h.config)
	if err != nil {
		return
	}
	if err = h.setupConfig(); err != nil {
		return
	}

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
	var b []byte
	b, err = readFile(h.rootPath, DNAHashFileName)
	if err == nil {
		h.dnaHash, err = NewHash(string(b))
		if err != nil {
			return
		}
		// @TODO compare value from file to actual hash
	}

	if h.chain.Length() > 0 {
		h.agentHash = h.chain.Headers[1].EntryLink
		_, topHeader := h.chain.TopType(AgentEntryType)
		h.agentTopHash = topHeader.EntryLink
	}
	if err = h.Prepare(); err != nil {
		return
	}

	hP = &h
	return
}

// gen calls a make function which should build the holochain structure and supporting files
func gen(root string, makeH func(root string) (hP *Holochain, err error)) (h *Holochain, err error) {
	if dirExists(root) {
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

	if err := os.MkdirAll(h.DBPath(), os.ModePerm); err != nil {
		return nil, err
	}

	h.chain, err = NewChainFromFile(h.hashSpec, filepath.Join(h.DBPath(), StoreFileName))
	if err != nil {
		return nil, err
	}

	//p := filepath.Join(root, ChainDNADir, DNAFileName + "." + h.encodingFormat)
	//if fileExists(p) {
	//	return nil, mkErr(p + " already exists")
	//}
	//f, err := os.Create(p)
	//if err != nil {
	//	return nil, err
	//}
	//defer f.Close()
	//err = h.EncodeDNA(f)

	if err != nil {
		return nil, err
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

func makeConfig(h *Holochain, s *Service) (err error) {
	h.config = Config{
		Port:            DefaultPort,
		PeerModeDHTNode: s.Settings.DefaultPeerModeDHTNode,
		PeerModeAuthor:  s.Settings.DefaultPeerModeAuthor,
		BootstrapServer: s.Settings.DefaultBootstrapServer,
		Loggers: Loggers{
			App:        Logger{Format: "%{color:cyan}%{message}", Enabled: true},
			DHT:        Logger{Format: "%{color:yellow}%{time} DHT: %{message}"},
			Gossip:     Logger{Format: "%{color:blue}%{time} Gossip: %{message}"},
			TestPassed: Logger{Format: "%{color:green}%{message}", Enabled: true},
			TestFailed: Logger{Format: "%{color:red}%{message}", Enabled: true},
			TestInfo:   Logger{Format: "%{message}", Enabled: true},
		},
	}

	val := os.Getenv("HOLOCHAINCONFIG_PORT")
	if val != "" {
		Debugf("makeConfig: using environment variable to set port to: %s", val)
		h.config.Port, err = strconv.Atoi(val)
		if err != nil {
			return err
		}

		if IsDebugging() {
			fmt.Printf("HC: service.go: makeConfig: using environment variable to set port to:            %v\n", val)
		}

	}
	val = os.Getenv("HOLOCHAINCONFIG_BOOTSTRAP")
	if val != "" {
		if val == "_" {
			val = ""
		}
		Debugf("makeConfig: using environment variable to set bootstrap server to: %s", val)
		h.config.BootstrapServer = val

		if val == "" {
			val = "NO BOOTSTRAP SERVER"
		}
		if IsDebugging() {
			fmt.Printf("HC: service.go: makeConfig: using environment variable to set bootstrapServer to: %v\n", val)
		}

	}
	val = os.Getenv("HOLOCHAINCONFIG_ENABLEMDNS")
	if val != "" {
		Debugf("makeConfig: using environment variable to set enableMDNS to: %s", val)
		h.config.EnableMDNS = val == "true"

		if IsDebugging() {
			fmt.Printf("HC: service.go: makeConfig: using environment variable to set enableMDNS to:      %v\n", val)
		}
	}
	val = os.Getenv("HOLOCHAINCONFIG_LOGPREFIX")
	if val != "" {
		Debugf("makeConfig: using environment variable to set log prefix to: %s", val)
		h.config.Loggers.App.Format = val + h.config.Loggers.App.Format
		h.config.Loggers.DHT.Format = val + h.config.Loggers.DHT.Format
		h.config.Loggers.Gossip.Format = val + h.config.Loggers.Gossip.Format
		h.config.Loggers.TestPassed.Format = val + h.config.Loggers.TestPassed.Format
		h.config.Loggers.TestFailed.Format = val + h.config.Loggers.TestFailed.Format
		h.config.Loggers.TestInfo.Format = val + h.config.Loggers.TestInfo.Format

		if IsDebugging() {
			fmt.Printf("HC: service.go: makeConfig: using environment variable to set logPrefix to:       %v\n", val)
		}
	}

	p := filepath.Join(h.rootPath, ConfigFileName+"."+h.encodingFormat)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = Encode(f, h.encodingFormat, &h.config); err != nil {
		return
	}
	if err = h.setupConfig(); err != nil {
		return
	}
	return
}

// GenDev generates starter holochain DNA files from which to develop a chain
func (s *Service) GenDev(root string, format string) (hP *Holochain, err error) {
	hP, err = gen(root, func(root string) (hP *Holochain, err error) {
		var agent Agent
		agent, err = NewAgent(LibP2P, "Example Agent <example@example.com")
		if err != nil {
			return
		}

		err = agent.GenKeys(bytes.NewBuffer([]byte("fixed seed 012345678901234567890123456789")))
		if err != nil {
			return
		}

		var zomes []Zome

		h := NewHolochain(agent, root, format, zomes...)
		if err = h.mkChainDirs(); err != nil {
			return nil, err
		}
		if err = makeConfig(&h, s); err != nil {
			return
		}

		//fmt.Print("\nGenDev creating new holochain in ", h.rootPath)

		propertiesSchemaFile := "properties_schema.json"

		dna := h.nucleus.dna
		dnaFile := DNAFile{
			Name:                 filepath.Base(root),
			UUID:                 dna.UUID,
			RequiresVersion:      dna.Version,
			DHTConfig:            dna.DHTConfig,
			Progenitor:           dna.Progenitor,
			PropertiesSchemaFile: propertiesSchemaFile,
		}

		zygoZomeName := "zySampleZome"
		jsZomeName := "jsSampleZome"

		dnaFile.Zomes = []ZomeFile{
			{
				Name:         zygoZomeName,
				CodeFile:     zygoZomeName + ".zy",
				Description:  "this is a zygomas test zome",
				RibosomeType: ZygoRibosomeType,
				BridgeFuncs:  []string{"testStrFn1"},
				Entries: []EntryDefFile{
					{Name: "evenNumbers", DataFormat: DataFormatRawZygo, Sharing: Public},
					{Name: "primes", DataFormat: DataFormatJSON, Sharing: Public, SchemaFile: "primes.json"},
					{Name: "profile", DataFormat: DataFormatJSON, Sharing: Public, SchemaFile: "profile.json"},
				},
				Functions: []FunctionDef{
					{Name: "getDNA", CallingType: STRING_CALLING},
					{Name: "addEven", CallingType: STRING_CALLING, Exposure: PUBLIC_EXPOSURE},
					{Name: "addPrime", CallingType: JSON_CALLING, Exposure: PUBLIC_EXPOSURE},
					{Name: "testStrFn1", CallingType: STRING_CALLING},
					{Name: "testStrFn2", CallingType: STRING_CALLING},
					{Name: "testJsonFn1", CallingType: JSON_CALLING},
					{Name: "testJsonFn2", CallingType: JSON_CALLING},
				},
			},
			{
				Name:         jsZomeName,
				CodeFile:     jsZomeName + ".js",
				Description:  "this is a javascript test zome",
				RibosomeType: JSRibosomeType,
				BridgeFuncs:  []string{"getProperty"},
				Entries: []EntryDefFile{
					{Name: "oddNumbers", DataFormat: DataFormatRawJS, Sharing: Public},
					{Name: "profile", DataFormat: DataFormatJSON, Sharing: Public, SchemaFile: "profile.json"},
					{Name: "rating", DataFormat: DataFormatLinks},
					{Name: "secret", DataFormat: DataFormatString},
				},
				Functions: []FunctionDef{
					{Name: "getProperty", CallingType: STRING_CALLING, Exposure: PUBLIC_EXPOSURE},
					{Name: "addOdd", CallingType: STRING_CALLING, Exposure: PUBLIC_EXPOSURE},
					{Name: "addProfile", CallingType: JSON_CALLING, Exposure: PUBLIC_EXPOSURE},
					{Name: "testStrFn1", CallingType: STRING_CALLING},
					{Name: "testStrFn2", CallingType: STRING_CALLING},
					{Name: "testJsonFn1", CallingType: JSON_CALLING},
					{Name: "testJsonFn2", CallingType: JSON_CALLING},
				}},
		}

		dnaFile.Properties = map[string]string{
			"description": "a bogus test holochain",
			"language":    "en"}

		dnaPath := filepath.Join(h.DNAPath(), "dna."+format)
		//fmt.Printf("\nGenDev writing new DNA to: %s", dnaPath)
		var f *os.File
		f, err = os.Create(dnaPath)
		if err != nil {
			return
		}
		err = Encode(f, format, &dnaFile)
		f.Close()
		if err != nil {
			return
		}

		propertiesSchema := `{
	"title": "Properties Schema",
	"type": "object",
	"properties": {
		"description": {
			"type": "string"
		},
		"language": {
			"type": "string"
		}
	}
}`

		if err = writeFile([]byte(propertiesSchema), h.DNAPath(), propertiesSchemaFile); err != nil {
			return
		}

		profileSchema := `{
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

		primesSchema := `{
	"title": "Prime Schema",
	"type": "object",
	"properties": {
		"prime": {
			"type": "integer"
		}
	},
	"required": ["prime"]
}`

		zygoCode := `
(defn testStrFn1 [x] (concat "result: " x))
(defn testStrFn2 [x] (+ (atoi x) 2))
(defn testJsonFn1 [x] (begin (hset x output: (* (-> x input:) 2)) x))
(defn testJsonFn2 [x] (unjson (raw "[{\"a\":\"b\"}]"))) (defn getDNA [x] App_DNA_Hash)
(defn addEven [x] (commit "evenNumbers" x))
(defn addPrime [x] (commit "primes" x))
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
(defn genesis [] true)
(defn bridgeGenesis [] (begin (debug "bridge genesis debug output")  true))
(defn receive [from message]
	(hash pong: (hget message %ping)))
`
		if err = os.MkdirAll(filepath.Join(h.DNAPath(), zygoZomeName), os.ModePerm); err != nil {
			return nil, err
		}
		if err = writeFile([]byte(zygoCode), h.DNAPath(), zygoZomeName, zygoZomeName+".zy"); err != nil {
			return
		}
		if err = writeFile([]byte(profileSchema), h.DNAPath(), zygoZomeName, "profile.json"); err != nil {
			return
		}
		if err = writeFile([]byte(primesSchema), h.DNAPath(), zygoZomeName, "primes.json"); err != nil {
			return
		}

		jsCode := `
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

function genesis() {return true}
function bridgeGenesis() {return true}

function receive(from,message) {
  // send back a pong message of what came in the ping message!
  return {pong:message.ping}
}

`

		if err = os.MkdirAll(filepath.Join(h.DNAPath(), jsZomeName), os.ModePerm); err != nil {
			return nil, err
		}
		if err = writeFile([]byte(jsCode), h.DNAPath(), jsZomeName, jsZomeName+".js"); err != nil {
			return
		}
		if err = writeFile([]byte(profileSchema), h.DNAPath(), jsZomeName, "profile.json"); err != nil {
			return
		}

		fixtures := [8]TestData{
			{
				Zome:   "zySampleZome",
				FnName: "addEven",
				Input:  "2",
				Output: "%h%"},
			{
				Zome:   "zySampleZome",
				FnName: "addEven",
				Input:  "4",
				Output: "%h%"},
			{
				Zome:   "zySampleZome",
				FnName: "addEven",
				Input:  "5",
				Err:    "Error calling 'commit': Invalid entry: 5"},
			{
				Zome:   "zySampleZome",
				FnName: "addPrime",
				Input:  "{\"prime\":7}",
				Output: "\"%h%\""}, // quoted because return value is json
			{
				Zome:   "zySampleZome",
				FnName: "addPrime",
				Input:  "{\"prime\":4}",
				Err:    `Error calling 'commit': Invalid entry: {"prime":4}`},
			{
				Zome:   "jsSampleZome",
				FnName: "addProfile",
				Input:  `{"firstName":"Art","lastName":"Brock"}`,
				Output: `"%h%"`},
			{
				Zome:   "zySampleZome",
				FnName: "getDNA",
				Input:  "",
				Output: "%dna%"},
			{
				Zome:     "zySampleZome",
				FnName:   "getDNA",
				Input:    "",
				Err:      "function not available",
				Exposure: PUBLIC_EXPOSURE,
			},
		}

		fixtures2 := [3]TestData{
			{
				Zome:   "jsSampleZome",
				FnName: "addOdd",
				Input:  "7",
				Output: "%h%"},
			{
				Zome:   "jsSampleZome",
				FnName: "addOdd",
				Input:  "2",
				Err:    "Invalid entry: 2"},
			{
				Zome:   "jsSampleZome",
				Input:  "unexposed(\"this is a\")",
				Output: "this is a fish",
				Raw:    true,
			},
		}

		for fileName, fileText := range SampleUI {
			if err = writeFile([]byte(fileText), h.UIPath(), fileName); err != nil {
				return
			}
		}

		testPath := filepath.Join(root, "test")
		if err = os.MkdirAll(testPath, os.ModePerm); err != nil {
			return nil, err
		}

		// write out the tests
		for i, d := range fixtures {
			fn := fmt.Sprintf("test_%d.json", i)
			var j []byte
			t := []TestData{d}
			j, err = json.Marshal(t)
			if err != nil {
				return
			}
			if err = writeFile(j, testPath, fn); err != nil {
				return
			}
		}

		// also write out some grouped tests
		fn := "grouped.json"
		var j []byte
		j, err = json.Marshal(fixtures2)
		if err != nil {
			return
		}
		if err = writeFile(j, testPath, fn); err != nil {
			return
		}

		// also write out some scenarios
		var scenarioPath string
		for _, scenario := range []string{"authorize", "fail"} {
			scenarioPath = filepath.Join(testPath, scenario)
			if err = os.MkdirAll(scenarioPath, os.ModePerm); err != nil {
				return nil, err
			}
			if err = writeFile([]byte(`{"bogus":"bogus test!"}`), scenarioPath, "requester.json"); err != nil {
				return
			}
			if err = writeFile([]byte(`{"bogus":"bogus test!"}`), scenarioPath, "responder.json"); err != nil {
				return
			}
		}

		//fmt.Printf("\nGenDev done generating. Loading now..")

		hP, err = s.Load(dnaFile.Name)
		return
	})
	return
}

// Clone copies DNA files from a source directory
// bool new indicates if this clone should create a new DNA (when true) or act as a Join
func (s *Service) Clone(srcPath string, root string, agent Agent, new bool) (err error) {
	_, err = gen(root, func(root string) (hP *Holochain, err error) {
		var h Holochain
		srcDNAPath := filepath.Join(srcPath, ChainDNADir)
		//fmt.Printf("\n%s\n", srcDNAPath)
		format, err := findDNA(srcDNAPath)
		if err != nil {
			return
		}

		dna, err := s.LoadDNA(srcDNAPath, DNAFileName, format)
		if err != nil {
			return
		}

		h.nucleus = NewNucleus(&h, dna)
		h.encodingFormat = format
		h.rootPath = root

		// create the DNA directory and copy
		if err := os.MkdirAll(h.DNAPath(), os.ModePerm); err != nil {
			return nil, err
		}

		//fmt.Printf("dna: agent, err: %s\n", agent, err)
		// TODO verify identity against schema?
		h.agent = agent

		// once the agent is set up we can calculate the id
		h.nodeID, h.nodeIDStr, err = agent.NodeID()
		if err != nil {
			return
		}

		// make a config file
		if err = makeConfig(&h, s); err != nil {
			return
		}

		if new {
			h.nucleus.dna.NewUUID()

			// use the path as the name
			h.nucleus.dna.Name = filepath.Base(root)

			// change the progenitor to self because this is a clone!
			var pk []byte
			pk, err = agent.PubKey().Bytes()
			if err != nil {
				return
			}
			h.nucleus.dna.Progenitor = Progenitor{Identity: string(agent.Identity()), PubKey: pk}
		}

		// save out the DNA file
		if err = s.SaveDNAFile(h.rootPath, h.nucleus.dna, h.encodingFormat, true); err != nil {
			return
		}

		// copy any UI files
		srcUIPath := filepath.Join(srcPath, ChainUIDir)
		if dirExists(srcUIPath) {
			if err = CopyDir(srcUIPath, h.UIPath()); err != nil {
				return
			}
		}

		// copy any test files
		srcTestDir := filepath.Join(srcPath, ChainTestDir)
		if dirExists(srcTestDir) {
			if err = CopyDir(srcTestDir, filepath.Join(root, ChainTestDir)); err != nil {
				return
			}
		}

		//fmt.Printf("srcTestDir: %s, err: %s\n", srcTestDir, err)

		hP = &h

		return
	})
	return
}

// GenChain adds the genesis entries to a newly cloned or joined chain
func (s *Service) GenChain(name string) (h *Holochain, err error) {
	h, err = s.Load(name)
	if err != nil {
		return
	}
	err = h.Activate()
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
		list = "installed holochains: "
		for _, k := range keys {
			id := chains[k].DNAHash()
			var sid = "<not-started>"
			if id.String() != "" {
				sid = id.String()
			}
			list += fmt.Sprintf("    %v %v\n", k, sid)
		}

	} else {
		list = "no installed chains"
	}
	return
}

// SaveDNAFile writes out holochain DNA to files
func (s *Service) SaveDNAFile(root string, dna *DNA, encodingFormat string, overwrite bool) (err error) {
	dnaPath := filepath.Join(root, ChainDNADir)
	p := filepath.Join(dnaPath, DNAFileName+"."+encodingFormat)
	if !overwrite && fileExists(p) {
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
		if err = writeFile([]byte(z.Code), zpath, z.Name+suffixByRibosomeType(z.RibosomeType)); err != nil {
			return
		}

		zomeFile := ZomeFile{Name: z.Name,
			Description:  z.Description,
			CodeFile:     z.CodeFileName(),
			RibosomeType: z.RibosomeType,
			Functions:    z.Functions,
			BridgeFuncs:  z.BridgeFuncs,
			BridgeTo:     z.BridgeTo,
		}

		for _, e := range z.Entries {
			entryDefFile := EntryDefFile{
				Name:       e.Name,
				DataFormat: e.DataFormat,
				Sharing:    e.Sharing,
			}
			if e.DataFormat == DataFormatJSON && e.Schema != "" {
				entryDefFile.SchemaFile = e.Name + ".json"
				if err = writeFile([]byte(e.Schema), zpath, e.Name+".json"); err != nil {
					return
				}
			}

			zomeFile.Entries = append(zomeFile.Entries, entryDefFile)
		}
		dnaFile.Zomes = append(dnaFile.Zomes, zomeFile)
	}

	if dna.PropertiesSchema != "" {
		if err = writeFile([]byte(dna.PropertiesSchema), dnaPath, "properties_schema.json"); err != nil {
			return
		}
	}

	err = Encode(f, encodingFormat, dnaFile)
	return
}

func IsDebugging() bool {
	return strings.ToLower(os.Getenv("DEBUG")) == "true" || os.Getenv("DEBUG") == "1"
}

// SaveScaffold writes out a holochain application based on scaffold file to path
func (service *Service) SaveScaffold(reader io.Reader, path string, newUUID bool) (scaffold *Scaffold, err error) {
	scaffold, err = LoadScaffold(reader)
	if err != nil {
		return
	}

	dna := &scaffold.DNA
	err = MakeDirs(path)
	if err != nil {
		return
	}
	if newUUID {
		dna.NewUUID()
	}
	err = service.SaveDNAFile(path, dna, "json", false)
	if err != nil {
		return
	}

	testPath := filepath.Join(path, ChainTestDir)
	for _, test := range scaffold.Tests {
		if err = writeFile([]byte(test.Value), testPath, test.Name+".json"); err != nil {
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
