// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// command line interface to developing and testing holochain applications

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	holo "github.com/maackle/holochain-proto"
	. "github.com/maackle/holochain-proto/apptest"
	"github.com/maackle/holochain-proto/cmd"
	"github.com/maackle/holochain-proto/ui"
	"github.com/urfave/cli"
	// fsnotify	"github.com/fsnotify/fsnotify"
	//spew "github.com/davecgh/go-spew/spew"
)

const (
	defaultUIPort      = "4141"
	scenarioStartDelay = 1

	defaultSpecsFile = "bridge_specs.json"
)

var debug, appInitialized, verbose, keepalive bool
var keepaliveCleanup func()
var rootPath, devPath, name string
var bridgeSpecsFile string
var scenarioConfig *holo.TestConfig

// flags for holochain config generation
var dhtPort, logPrefix, bootstrapServer string
var mdns bool = true
var upnp bool

// meta flags for program flow control
var syncPausePath string
var syncPauseUntil int

type MutableContext struct {
	str map[string]string
	obj map[string]interface{}
}

var mutableContext MutableContext

var lastRunContext *cli.Context

var sysUser *user.User

// TODO: move these into cmd module

func appCheck(devPath string) error {
	if !appInitialized {
		return cmd.MakeErr(nil, fmt.Sprintf("%s doesn't look like a holochain app (missing dna).  See 'hcdev init -h' for help on initializing an app.", devPath))
	}
	return nil
}
func setupApp() (app *cli.App) {

	// set default values so we can call this multiple time for testing
	debug = false
	appInitialized = false
	rootPath = ""
	devPath = ""
	name = ""
	mutableContext = MutableContext{map[string]string{}, map[string]interface{}{}}

	var err error
	sysUser, err = user.Current()
	if err != nil {
		panic(err)
	}

	app = cli.NewApp()
	app.Name = "hcdev"
	app.Usage = "holochain dev command line tool"
	app.Version = fmt.Sprintf("0.0.6 (holochain %s)", holo.VersionStr)

	var service *holo.Service
	var serverID, agentID, identity string

	var scenarioTmpDir = "hcdev_scenario_test_nodes_" + sysUser.Username

	var dumpScenario string
	var dumpTest bool
	var start int

	var bridgeAppTmpFilePath string

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "debugging output",
			Destination: &debug,
		},
		cli.BoolFlag{
			Name:        "verbose",
			Usage:       "verbose output",
			Destination: &verbose,
		},
		cli.BoolFlag{
			Name:        "keepalive",
			Usage:       "don't end hcdev process upon completion of work",
			Destination: &keepalive,
		},
		cli.StringFlag{
			Name:        "execpath",
			Usage:       "path to holochain dev execution directory (default: ~/.holochaindev)",
			Destination: &rootPath,
		},
		cli.StringFlag{
			Name:        "path",
			Usage:       "path to chain source definition directory (default: current working dir)",
			Destination: &devPath,
		},
		cli.StringFlag{
			Name:        "DHTport",
			Usage:       fmt.Sprintf("port to use for the holochain DHT and node-to-node communication (defaut: %d)", holo.DefaultDHTPort),
			Destination: &dhtPort,
		},
		cli.BoolTFlag{
			Name:        "mdns",
			Usage:       "whether to use mdns for local peer discovery (default: true)",
			Destination: &mdns,
		},
		cli.BoolFlag{
			Name:        "upnp",
			Usage:       "whether to use UPnP for creating a NAT port mapping (default: false)",
			Destination: &upnp,
		},
		cli.StringFlag{
			Name:        "logPrefix",
			Usage:       "the prefix to put at the front of log messages",
			Destination: &logPrefix,
		},
		cli.StringFlag{
			Name:        "bootstrapServer",
			Usage:       "url of bootstrap server or '_' for none",
			Destination: &bootstrapServer,
		},
		cli.StringFlag{
			Name:        "bridgeSpecs",
			Usage:       fmt.Sprintf("path to bridge specs file (default: %s)", defaultSpecsFile),
			Destination: &bridgeSpecsFile,
		},
		cli.StringFlag{
			Name:        "serverID",
			Usage:       "server identifier for multi-server scenario testing",
			Destination: &serverID,
		},
		cli.StringFlag{
			Name:        "agentID",
			Usage:       "value to use for the agent identity (automatically set in scenario testing)",
			Destination: &agentID,
		},
	}

	var dumpChain, dumpDHT, initTest, fromDevelop, benchmarks, json bool
	var clonePath, appPackagePath, cloneExample, outputDir, fromBranch, dumpFormat string

	app.Commands = []cli.Command{
		{
			Name:    "init",
			Aliases: []string{"i"},
			Usage:   "initialize a holochain app directory: use default, from an appPackage file or clone from another app",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "test",
					Usage:       "initialize built-in testing app",
					Destination: &initTest,
				},
				cli.StringFlag{
					Name:        "clone",
					Usage:       "path from which to clone the app",
					Destination: &clonePath,
				},
				cli.StringFlag{
					Name:        "package",
					Usage:       "path to an app package file from which to initialize the app",
					Destination: &appPackagePath,
				},
				cli.StringFlag{
					Name:        "cloneExample",
					Usage:       "example from github.com/holochain to clone from",
					Destination: &cloneExample,
				},
				cli.StringFlag{
					Name:        "fromBranch",
					Usage:       "specify branch to use with cloneExample",
					Destination: &fromBranch,
				},
				cli.BoolFlag{
					Name:        "fromDevelop",
					Usage:       "specify that cloneExample should use the 'develop' branch",
					Destination: &fromDevelop,
				},
			},
			ArgsUsage: "<name>",
			Action: func(c *cli.Context) error {
				var name string
				args := c.Args()
				if len(args) != 1 {
					if cloneExample != "" {
						name = cloneExample
					} else {
						return cmd.MakeErr(c, "expecting app name as single argument")
					}
				}
				flags := 0
				if clonePath != "" {
					flags += 1
				}
				if appPackagePath != "" {
					flags += 1
				}
				if initTest {
					flags += 1
				}
				if flags > 1 {
					return cmd.MakeErr(c, " options are mutually exclusive, please choose just one.")
				}
				if name == "" {
					name = args[0]
				}
				if filepath.IsAbs(name) {
					devPath = name
					name = filepath.Base(name)
				} else {
					devPath = filepath.Join(devPath, name)
				}

				info, err := os.Stat(devPath)
				if err == nil && info.Mode().IsDir() {
					return cmd.MakeErr(c, fmt.Sprintf("%s already exists", devPath))
				}

				encodingFormat := "json"
				if initTest {
					fmt.Printf("initializing test app as %s\n", name)
					format := "json"
					if len(c.Args()) == 2 {
						format = c.Args()[1]
						if !(format == "json" || format == "yaml" || format == "toml") {
							return cmd.MakeErr(c, "format must be one of yaml,toml,json")

						}
					}
					_, err := service.MakeTestingApp(devPath, "json", holo.SkipInitializeDB, holo.CloneWithNewUUID, nil)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
				} else if clonePath != "" {

					// build the app by cloning from another app
					info, err := os.Stat(clonePath)
					if err != nil {
						dir, _ := cmd.GetCurrentDirectory()
						return cmd.MakeErr(c, fmt.Sprintf("ClonePath:%s/'%s' %s", dir, clonePath, err.Error()))
					}

					if !info.Mode().IsDir() {
						return cmd.MakeErr(c, "-clone flag expects a directory to clone from")
					}
					fmt.Printf("cloning %s from %s\n", name, clonePath)
					err = doClone(service, clonePath, devPath)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
				} else if cloneExample != "" {
					tmpCopyDir, err := ioutil.TempDir("", fmt.Sprintf("holochain.example.%s", cloneExample))
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
					defer os.RemoveAll(tmpCopyDir)
					err = os.Chdir(tmpCopyDir)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
					if fromDevelop {
						fromBranch = "develop"
					}
					command := exec.Command("git", "clone", fmt.Sprintf("git://github.com/holochain/%s.git", cloneExample))
					out, err := command.CombinedOutput()
					fmt.Printf("git: %s\n", string(out))
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}

					if fromBranch != "" {
						err = os.Chdir(filepath.Join(tmpCopyDir, cloneExample))
						if err != nil {
							return cmd.MakeErrFromErr(c, err)
						}
						command := exec.Command("git", "checkout", fromBranch)
						out, err := command.CombinedOutput()
						fmt.Printf("git: %s\n", string(out))
						if err != nil {
							return cmd.MakeErrFromErr(c, err)
						}
					}

					clonePath := filepath.Join(tmpCopyDir, cloneExample)
					fmt.Printf("cloning %s from github.com/holochain/%s\n", name, cloneExample)
					err = doClone(service, clonePath, devPath)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}

				} else if appPackagePath != "" {
					// build the app from the appPackage
					_, err := cmd.UpackageAppPackage(service, appPackagePath, devPath, name, encodingFormat)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}

					fmt.Printf("initialized %s from appPackage:%s\n", devPath, appPackagePath)
				} else {

					// build empty app template
					err := holo.MakeDirs(devPath)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
					appPackageReader := bytes.NewBuffer([]byte(holo.BasicTemplateAppPackage))

					var agent holo.Agent
					agent, err = holo.LoadAgent(rootPath)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}

					var appPackage *holo.AppPackage
					appPackage, err = service.SaveFromAppPackage(appPackageReader, devPath, name, agent, holo.BasicTemplateAppPackageFormat, encodingFormat, true)

					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
					fmt.Printf("initialized empty application to %s with new UUID:%v\n", devPath, appPackage.DNA.UUID)
				}

				err = os.Chdir(devPath)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				return nil
			},
		},
		{
			Name:      "run-js",
			Aliases:   []string{},
			ArgsUsage: "[zome] | [zome] [filename.js]",
			Usage:     "Run arbitrary code within the JavaScript ribosome (useful for testing)",
			Flags: []cli.Flag{

			},
			Action: func(c *cli.Context) error {
				args := c.Args()

				var h *holo.Holochain
				var js string

				if len(args) < 1 || len(args) > 2 {
					return cmd.MakeErr(c, "Must run with one or two arguments. See usage.")
				}

				h, _ = getHolochain(c, service, identity)
				SetupForPureJSTest(h, false, []holo.BridgeApp{})

				zomeName := args[0]
				n, _, _ := h.MakeRibosome(zomeName)

				if len(args) == 1 {
					reader := bufio.NewReader(os.Stdin)
					buf := new(bytes.Buffer)
					buf.ReadFrom(reader)
					js = buf.String()
				} else if len(args) == 2 {
					fileName := args[1]
					buf, err := holo.ReadFile(".", fileName)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
					js = string(buf[:])
				}

				_, err := n.RunWithTimers(js)

				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				} else {
					fmt.Println("done.")
				}

				return nil
			},
		},
		{
			Name:      "test",
			Aliases:   []string{"t"},
			ArgsUsage: "no args run's all stand-alone | [test file prefix] | [scenario] [role]",
			Usage:     "run chain's stand-alone or scenario tests",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "syncPausePath",
					Usage:       "path to wait for multinode test sync",
					Destination: &syncPausePath,
				},
				cli.IntFlag{
					Name:        "syncPauseUntil",
					Usage:       "unix timestamp - sync tests to run at this time",
					Destination: &syncPauseUntil,
				},
				cli.BoolFlag{
					Name:        "benchmarks",
					Usage:       "calculate benchmarks during test",
					Destination: &benchmarks,
				},
				cli.StringFlag{
					Name:        "bridgeAppsFile",
					Usage:       "path to live bridging Apps (used internally when scenario testing)",
					Destination: &bridgeAppTmpFilePath,
				},
			},
			Action: func(c *cli.Context) error {
				holo.Debug("test: start")

				var err error
				if err = appCheck(devPath); err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				args := c.Args()
				var errs []error

				var h *holo.Holochain
				h, err = getHolochain(c, service, identity)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}
				holo.Debug("test: initialised holochain\n")

				if len(args) < 2 {
					var bridgeApps []BridgeAppForTests

					bridgeApps, err = getBridgeAppForTests(service, h.Agent())
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}

					if len(args) == 1 {
						errs = TestOne(h, args[0], bridgeApps, benchmarks)
					} else if len(args) == 0 {
						errs = Test(h, bridgeApps, benchmarks)
					} else {
						return cmd.MakeErr(c, "expected 0 args (run all stand-alone tests), 1 arg (a single stand-alone test) or 2 args (scenario and role)")
					}
				} else {
					var bridgeApps []holo.BridgeApp
					if bridgeAppTmpFilePath != "" {
						bridgeApps, err = getBridgeAppsFromTmpFile(bridgeAppTmpFilePath)
						if err != nil {
							return cmd.MakeErrFromErr(c, err)
						}
					}

					holo.Debug("test: scenario")

					scenario := args[0]
					role := args[1]
					holo.Debugf("test: scenario(%v, %v)\n", scenario, role)

					holo.Debugf("test: scenario(%v, %v): paused at: %v\n", scenario, role, time.Now())

					if syncPauseUntil != 0 {
						// IntFlag converts the string into int64 anyway. This explicit conversion is valid
						time.Sleep(cmd.GetDuration_fromUnixTimestamp(int64(syncPauseUntil)))
					}
					holo.Debugf("test: scenario(%v, %v): continuing at: %v\n", scenario, role, time.Now())
					pairs := map[string]string{"%server%": serverID}

					// The clone id is put into the identity by scenario call so we get
					// out with this regex
					re := regexp.MustCompile(`.*.([0-9]+)@.*`)
					x := re.FindStringSubmatch(string(h.Agent().Identity()))
					var clone string
					if len(x) > 0 {
						clone = x[1]
						pairs["%clone%"] = clone
					}

					host := getHostName(serverID)
					err = addRolesToPairs(h, scenario, host, pairs)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}

					err, errs = TestScenario(h, scenario, role, pairs, benchmarks, bridgeApps)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
					//holo.Debugf("testScenario: h: %v\n", spew.Sdump(h))

				}

				var s string
				for _, e := range errs {
					s += e.Error()
				}
				if s != "" {
					return cmd.MakeErr(c, s)
				}
				return nil
			},
		},
		{
			Name:      "scenario",
			Aliases:   []string{"s"},
			Usage:     "run a scenario test",
			ArgsUsage: "scenario-name",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "outputDir",
					Usage:       "directory to send output",
					Destination: &outputDir,
				},
				cli.BoolFlag{
					Name:        "benchmarks",
					Usage:       "calculate benchmarks during scenario test",
					Destination: &benchmarks,
				},
			},
			Action: func(c *cli.Context) error {
				mutableContext.str["command"] = "scenario"

				if err := appCheck(devPath); err != nil {
					return err
				}

				args := c.Args()
				if len(args) != 1 {
					return cmd.MakeErr(c, "missing scenario name argument")
				}
				scenarioName := args[0]

				// get the holochain from the source that we are supposed to be testing
				h, err := getHolochain(c, service, identity)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				// get the bridgeApps
				var bridgeApps []BridgeAppForTests
				bridgeApps, err = getBridgeAppForTests(service, h.Agent())
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				var bridgeAppsTmpfileName string
				if len(bridgeApps) > 0 {
					bridgeAppsTmpfileName, err = saveBridgeAppsToTmpFile(bridgeApps)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
				}

				//Spin up the bridgeApps
				var bridgeAppServers []*ui.WebServer
				for _, app := range bridgeApps {
					var bridgeAppServer *ui.WebServer
					bridgeAppServer, err = StartBridgeApp(app.H, app.BridgeApp.Port)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
					bridgeAppServers = append(bridgeAppServers, bridgeAppServer)
				}

				// mutableContext.obj["initialHolochain"] = h
				testScenarioList, err := holo.GetTestScenarios(h)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}
				mutableContext.obj["testScenarioList"] = &testScenarioList

				// confirm the user chosen scenario name
				//   TODO add this to code completion
				if _, ok := testScenarioList[scenarioName]; !ok {
					return cmd.MakeErr(c, "source argument is not directory in /test. scenario name must match directory name")
				}
				mutableContext.str["testScenarioName"] = scenarioName

				// get list of roles
				roleList, err := holo.GetTestScenarioRoles(h, scenarioName)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}
				mutableContext.obj["testScenarioRoleList"] = &roleList

				// run a bunch of hcdev test processes. Separate temp folder by username in case
				// multiple users on the same machine are running tests
				rootExecDir, err := cmd.MakeTmpDir(scenarioTmpDir)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}
				secondsFromNowPlusDelay := cmd.GetUnixTimestamp_secondsFromNow(scenarioStartDelay)

				scenarioPath := filepath.Join(h.TestPath(), scenarioName)

				scenarioConfig, err = holo.LoadTestConfig(scenarioPath)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				if outputDir != "" {
					err = os.MkdirAll(outputDir, os.ModePerm)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
				}

				for roleIndex, roleName := range roleList {
					holo.Debugf("scenario: forRole(%v): start\n\n", roleName)

					// HOLOCHAINCONFIG_DHTPORT       = FindSomeAvailablePort
					// HOLOCHAINCONFIG_ENABLEMDNS = "true" or HOLOCHAINCONFIG_BOOTSTRAP = "ip[localhost]:port[3142]
					// HCLOG_PREFIX  = role

					clones := 1

					for _, clone := range scenarioConfig.Clone {
						if clone.Role == roleName {
							clones = clone.Number
							break
						}
					}

					// if the bootstrapServer flag isn't set we assume this is a local scenario
					// test so we set the flag "_" the use no bootstrap
					if bootstrapServer == "" {
						bootstrapServer = "_"
					}

					// check to see if there's a bridge config for the role
					scenarioBridgeSpecs := filepath.Join(scenarioPath, "_"+roleName+"_"+defaultSpecsFile)
					holo.Debugf("scenario: looking for bridgeSpecs:%v", scenarioBridgeSpecs)
					if !holo.FileExists(scenarioBridgeSpecs) {
						scenarioBridgeSpecs = bridgeSpecsFile
					}

					originalRoleName := roleName
					for count := 0; count < clones; count++ {
						freePort, err := cmd.GetFreePort()
						if err != nil {
							return cmd.MakeErrFromErr(c, err)
						}

						if clones > 1 {
							roleName = fmt.Sprintf("%s.%d", originalRoleName, count)

						}
						agentID = roleName
						if serverID != "" {
							roleName = serverID + "." + roleName
						}
						holo.Debugf("scenario: forRole(%v): port: %v\n\n", roleName, freePort)

						colorByNumbers := []string{"green", "blue", "yellow", "cyan", "magenta", "red"}

						logPrefix := "%{color:" + colorByNumbers[roleIndex%6] + "}" + roleName + ": "
						/* time doesn't work in prefix yet
						if outputDir != "" {
							logPrefix = "%{time}" + logPrefix
						}*/

						var upnpnat string
						if bootstrapServer == "_" {
							upnpnat = "false"
						} else {
							upnpnat = "true"
						}
						testCommand := exec.Command(
							"hcdev",
							"-path="+devPath,
							"-execpath="+filepath.Join(rootExecDir, roleName),
							"-DHTport="+strconv.Itoa(freePort),
							fmt.Sprintf("-mdns=%v", mdns),
							"-upnp="+upnpnat,
							"-logPrefix="+logPrefix,
							"-serverID="+serverID,
							"-agentID="+agentID,
							fmt.Sprintf("-bootstrapServer=%v", bootstrapServer),
							fmt.Sprintf("-keepalive=%v", keepalive),

							"test",
							fmt.Sprintf("-bridgeAppsFile=%v", bridgeAppsTmpfileName),
							fmt.Sprintf("-benchmarks=%v", benchmarks),
							fmt.Sprintf("-syncPauseUntil=%v", secondsFromNowPlusDelay),
							scenarioName,
							originalRoleName,
						)

						mutableContext.obj["testCommand."+roleName] = &testCommand

						holo.Debugf("scenario: forRole(%v): testCommandPrepared: %v\n", roleName, testCommand)

						if outputDir != "" {
							f := filepath.Join(outputDir, roleName)
							df, err := os.Create(f)
							if err != nil {
								return cmd.MakeErrFromErr(c, err)
							}
							defer df.Close()
							testCommand.Stdout = df
							testCommand.Stderr = df
						} else {

							testCommand.Stdout = os.Stdout
							testCommand.Stderr = os.Stderr
						}
						testCommand.Start()

						holo.Debugf("scenario: forRole(%v): testCommandStarted\n", roleName)
					}
				}
				keepalive = true
				if len(bridgeApps) > 0 {
					keepaliveCleanup = func() {
						StopBridgeApps(bridgeAppServers)
						os.Remove(bridgeAppsTmpfileName)
					}
				}
				return nil
			},
		},
		{
			Name:      "web",
			Aliases:   []string{"serve", "w"},
			ArgsUsage: "[ui-port]",
			Usage:     fmt.Sprintf("serve a chain to the web on localhost:<ui-port> (default: %s)", defaultUIPort),
			Action: func(c *cli.Context) error {
				if err := appCheck(devPath); err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				h, err := getHolochain(c, service, agentID)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				bridgeApps, err := getBridgeAppForTests(service, h.Agent())
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				h.Close()
				h, err = service.GenChain(name)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				var port string
				if len(c.Args()) == 0 {
					port = defaultUIPort
				} else {
					port = c.Args()[0]
				}

				var ws *ui.WebServer
				ws, err = activate(h, port)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				var bridgeAppServers []*ui.WebServer
				bridgeAppServers, err = BuildBridges(h, port, bridgeApps)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}
				ws.Wait()
				// TODO call StopBridgeApps instead????
				for _, server := range bridgeAppServers {
					server.Stop()
				}

				return nil
			},
		},

		{
			Name:      "package",
			Aliases:   []string{"p"},
			ArgsUsage: "[output file]",
			Usage:     fmt.Sprintf("writes a package file of the dev path to file or stdout"),
			Action: func(c *cli.Context) error {

				var old *os.File
				if len(c.Args()) == 0 {
					old = os.Stdout // keep backup of the real stdout
					_, w, _ := os.Pipe()
					os.Stdout = w
				}

				if err := appCheck(devPath); err != nil {
					return err
				}
				h, err := getHolochain(c, service, identity)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}
				appPackage, err := service.MakeAppPackage(h)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				if len(c.Args()) == 0 {
					os.Stdout = old
					fmt.Print(string(appPackage))
				} else {
					err = holo.WriteFile(appPackage, c.Args().First())
				}
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}
				return nil
			},
		},

		{
			Name:      "dump",
			Aliases:   []string{"d"},
			ArgsUsage: "holochain-name",
			Usage:     "display a text dump of a chain after last 'web', 'test', or 'scenario'",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "chain",
					Destination: &dumpChain,
				},
				cli.BoolFlag{
					Name:        "dht",
					Destination: &dumpDHT,
				},
				cli.BoolFlag{
					Name:        "json",
					Destination: &json,
					Usage:       "Dump chain or dht as JSON string",
				},
				cli.IntFlag{
					Name:        "index",
					Destination: &start,
					Usage:       "starting index for dump (zero based)",
				},
				cli.BoolFlag{
					Name:        "test",
					Destination: &dumpTest,
				},
				cli.StringFlag{
					Name:        "scenario",
					Destination: &dumpScenario,
				},
				cli.StringFlag{
					Name:        "format",
					Destination: &dumpFormat,
					Usage:       "Dump format (string, json, dot)",
					Value:       "string",
				},
			},
			Action: func(c *cli.Context) error {

				if !dumpChain && !dumpDHT {
					dumpChain = true
				}

				var h *holo.Holochain
				var s *holo.Service
				var err error
				if dumpTest {
					panic("not implemented")
				} else if dumpScenario != "" {
					// the value is the role name which has it's own service for that role
					var d string
					d, err = cmd.GetTmpDir(scenarioTmpDir)
					if err == nil {
						s, err = holo.LoadService(filepath.Join(d, dumpScenario))
					}
				} else {
					// use default service
					s = service
				}
				if err == nil {
					h, err = s.Load(name)
					if err == nil {
						err = h.Prepare()
					}
				}
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				if !h.Started() {
					return cmd.MakeErr(c, "No data to dump, chain not yet initialized.")
				}

				dnaHash := h.DNAHash()
				if dumpChain {
					if json {
						dump, _ := h.Chain().JSON(start)
						fmt.Println(dump)
					} else if dumpFormat != "" {
						switch dumpFormat {
						case "string":
							fmt.Printf("Chain for: %s\n%v", dnaHash, h.Chain().Dump(start))
						case "dot":
							dump, _ := h.Chain().Dot(start)
							fmt.Println(dump)
						case "json":
							dump, _ := h.Chain().JSON(start)
							fmt.Println(dump)
						default:
							return cmd.MakeErr(c, "format must be one of dot,json,string")
						}
					} else {
						fmt.Printf("Chain for: %s\n%v", dnaHash, h.Chain().Dump(start))
					}
				}
				if dumpDHT {
					if json {
						dump, _ := h.DHT().JSON()
						fmt.Println(dump)
					} else {
						fmt.Printf("DHT for: %s\n%v", dnaHash, h.DHT().String())
					}
				}

				return nil
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		lastRunContext = c

		var err error

		if dhtPort != "" {
			err = os.Setenv("HOLOCHAINCONFIG_DHTPORT", dhtPort)
			if err != nil {
				return err
			}
		}
		if mdns {
			err = os.Setenv("HOLOCHAINCONFIG_ENABLEMDNS", "true")
			if err != nil {
				return err
			}
		}
		if upnp != true {
			err = os.Setenv("HOLOCHAINCONFIG_ENABLENATUPNP", "false")
			if err != nil {
				return err
			}
		}
		if logPrefix != "" {
			os.Setenv("HCLOG_PREFIX", logPrefix)
			if err != nil {
				return err
			}
		}
		if bootstrapServer != "" {
			os.Setenv("HOLOCHAINCONFIG_BOOTSTRAP", bootstrapServer)
			if err != nil {
				return err
			}
		}

		holo.Debugf("args:%v\n", c.Args())

		// hcdev always enables the app debugging, and the -debug flag enables the holochain debugging
		os.Setenv("HCLOG_APP_ENABLE", "1")
		if debug {
			os.Setenv("HCLOG_DHT_ENABLE", "1")
			os.Setenv("HCLOG_GOSSIP_ENABLE", "1")
			os.Setenv("HCLOG_DEBUG_ENABLE", "1")
		}
		holo.InitializeHolochain()

		if devPath == "" {
			devPath, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		name = filepath.Base(devPath)

		if cmd.IsAppDir(devPath) == nil {
			appInitialized = true
		}

		if rootPath == "" {
			rootPath = os.Getenv("HOLOPATHDEV")
			if rootPath == "" {
				userPath := sysUser.HomeDir
				rootPath = filepath.Join(userPath, holo.DefaultDirectoryName+"dev")
			}
		}
		identity = getIdentity(agentID, serverID)
		if !holo.IsInitialized(rootPath) {
			service, err = holo.Init(rootPath, holo.AgentIdentity(identity), holo.MakeTestSeed(identity))
			if err != nil {
				return err
			}
			fmt.Println("Holochain dev service initialized:")
			fmt.Printf("    %s directory created\n", rootPath)
			fmt.Printf("    defaults stored to %s\n", holo.SysFileName)
			fmt.Println("    key-pair generated")
			fmt.Printf("    using %s as default agent identity (stored to %s)\n", identity, holo.AgentFileName)

		} else {
			service, err = holo.LoadService(rootPath)
		}
		return err
	}

	app.Action = func(c *cli.Context) error {
		cli.ShowAppHelp(c)

		return nil
	}
	return

}

func main() {
	app := setupApp()
	err := app.Run(os.Args)
	var stop chan bool
	if keepalive {
		stop = make(chan bool, 1)
	}
	if keepalive && scenarioConfig != nil {
		go func() {
			time.Sleep(time.Second*(scenarioStartDelay+time.Duration(scenarioConfig.Duration)) + time.Second*scenarioStartDelay)
			stop <- true
		}()
	}
	if keepalive {
		<-stop
	}
	if keepaliveCleanup != nil {
		keepaliveCleanup()
	}
	if verbose {
		fmt.Printf("hcdev complete!\n")
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func getHolochain(c *cli.Context, service *holo.Service, identity string) (h *holo.Holochain, err error) {
	// clear out the previous chain data that was copied from the last test/run
	err = os.RemoveAll(filepath.Join(rootPath, name))
	if err != nil {
		return
	}
	var agent holo.Agent
	agent, err = holo.LoadAgent(rootPath)
	if err != nil {
		return
	}

	if identity != "" {
		holo.SetAgentIdentity(agent, holo.AgentIdentity(identity))
	}

	fmt.Printf("Copying chain to: %s\n", rootPath)
	h, err = service.Clone(devPath, filepath.Join(rootPath, name), agent, holo.CloneWithSameUUID, holo.InitializeDB)
	if err != nil {
		return
	}
	h.Close()

	h, err = service.Load(name)
	if err != nil {
		return
	}
	if verbose {
		fmt.Printf("Identity: %s\n", h.Agent().Identity())
		fmt.Printf("NodeID: %s\n", h.NodeIDStr())
	}
	return
}

// BridgeSpec describes an app to be bridged for dev
type BridgeSpec struct {
	Path                    string // path to the app to bridge to/from
	Side                    int    // what side of the bridge the dev app is (Bridge.Caller or Bridge.Callee)
	BridgeGenesisCallerData string // genesis data for the caller side
	BridgeGenesisCalleeData string // genesis data for the callee side
	Port                    string // only used if side == BridgeCallee
	BridgeZome              string // only used if side == BridgeCaller
}

// getBridgeAppsForTests builds up an array of bridged apps based on the dev values for bridging
func getBridgeAppForTests(service *holo.Service, agent holo.Agent) (bridgedApps []BridgeAppForTests, err error) {
	if bridgeSpecsFile == "_" {
		return
	}
	var specs []BridgeSpec
	specs, err = loadBridgeSpecs()
	if err != nil {
		return
	}
	for _, spec := range specs {
		var h *holo.Holochain
		h, err = setupBridgeApp(service, agent, spec.Path)
		if err != nil {
			return
		}
		if spec.Port == "" {
			var port int
			port, err = cmd.GetFreePort()
			if err != nil {
				return
			}
			spec.Port = fmt.Sprintf("%d", port)
		}
		bridgedApps = append(bridgedApps,
			BridgeAppForTests{
				H: h,
				BridgeApp: holo.BridgeApp{
					Name: h.Name(),
					DNA:  h.DNAHash(),
					Side: spec.Side,
					BridgeGenesisCallerData: spec.BridgeGenesisCallerData,
					BridgeGenesisCalleeData: spec.BridgeGenesisCalleeData,
					Port:       spec.Port,
					BridgeZome: spec.BridgeZome,
				},
			})
	}
	return
}

// setupBridgeApp clones the bridge app from source and loads it in preparation for actual bridging
func setupBridgeApp(service *holo.Service, agent holo.Agent, path string) (bridgeH *holo.Holochain, err error) {

	bridgeName := filepath.Base(path)

	os.Setenv("HOLOCHAINCONFIG_ENABLEMDNS", "true")
	os.Setenv("HOLOCHAINCONFIG_BOOTSTRAP", "_")
	os.Setenv("HCLOG_PREFIX", bridgeName+":")

	var freePort int
	freePort, err = cmd.GetFreePort()
	if err != nil {
		return
	}

	os.Setenv("HOLOCHAINCONFIG_DHTPORT", fmt.Sprintf("%d", freePort))

	fmt.Printf("Copying bridge chain %s to: %s\n", bridgeName, rootPath)
	// cleanup from previous time
	err = os.RemoveAll(filepath.Join(rootPath, bridgeName))
	if err != nil {
		return
	}
	_, err = service.Clone(path, filepath.Join(rootPath, bridgeName), agent, holo.CloneWithSameUUID, holo.InitializeDB)
	if err != nil {
		return
	}

	bridgeH, err = service.Load(bridgeName)
	if err != nil {
		return
	}

	_, err = bridgeH.GenChain()
	if err != nil {
		return
	}

	// clear the log prefix for the next load.
	os.Unsetenv("HCLOG_PREFIX")
	return
}

func activate(h *holo.Holochain, port string) (ws *ui.WebServer, err error) {
	fmt.Printf("Serving holochain with DNA hash:%v on port:%s\n", h.DNAHash(), port)
	err = h.Activate()
	if err != nil {
		return
	}
	h.StartBackgroundTasks()
	ws = ui.NewWebServer(h, port)
	ws.Start()
	return
}

func GetLastRunContext() (MutableContext, *cli.Context) {
	return mutableContext, lastRunContext
}

func doClone(s *holo.Service, clonePath, devPath string) (err error) {

	// TODO this is the bogus dev agent, really it should probably be someone else
	agent, err := holo.LoadAgent(rootPath)
	if err != nil {
		return
	}

	_, err = s.Clone(clonePath, devPath, agent, holo.CloneWithSameUUID, holo.SkipInitializeDB)
	if err != nil {
		return
	}
	return
}

func loadBridgeSpecs() (specs []BridgeSpec, err error) {
	if bridgeSpecsFile == "" {
		holo.Debug("no bridgeSpecs checking for default")
		if holo.FileExists(defaultSpecsFile) {
			bridgeSpecsFile = defaultSpecsFile
		}
	}
	if bridgeSpecsFile != "" {
		holo.Debugf("load bridgeSpecs:%s", bridgeSpecsFile)
		err = holo.DecodeFile(&specs, bridgeSpecsFile)
	}
	return
}

func getHostName(serverID string) (host string) {
	if serverID != "" {
		host = serverID
	} else {
		host, _ = os.Hostname()
	}
	if host == "" {
		host = "example.com"
	}
	return
}

func getIdentity(agentID, serverID string) (identity string) {
	var host, username string
	host = getHostName(serverID)

	if agentID != "" {
		username = agentID
	} else {
		username = sysUser.Username
	}
	if username == "" {
		username = "test"
	}

	identity = username + "@" + host
	return
}

func addRolesToPairs(h *holo.Holochain, scenario string, host string, pairs map[string]string) (err error) {

	var roles []string
	roles, err = holo.GetTestScenarioRoles(h, scenario)
	if err != nil {
		return
	}

	dir := filepath.Join(h.TestPath(), scenario)
	var config *holo.TestConfig
	config, err = holo.LoadTestConfig(dir)
	if err != nil {
		return
	}

	cloneRoles := make(map[string]holo.CloneSpec)
	for _, spec := range config.Clone {
		cloneRoles[spec.Role] = spec
	}

	for _, role := range roles {

		var testSet holo.TestSet
		testSet, err = holo.LoadTestFile(dir, role+".json")
		if err != nil {
			return
		}
		spec, isClone := cloneRoles[role]

		if testSet.Identity != "" {
			if isClone {
				err = fmt.Errorf("can't both clone and specify an identity: role %s", role)
				return
			}
			err = addRoleToPairs(h, role, testSet.Identity, pairs)
		} else {
			if isClone {
				origRole := role
				for i := 0; i < spec.Number; i++ {
					role = fmt.Sprintf("%s.%d", origRole, i)
					err = addRoleToPairs(h, role, fmt.Sprintf("%s@%s", role, host), pairs)
				}
			} else {
				err = addRoleToPairs(h, role, role+"@"+host, pairs)
			}
		}

	}
	return
}
func addRoleToPairs(h *holo.Holochain, role string, id string, pairs map[string]string) (err error) {
	var agent holo.Agent
	agent, err = holo.NewAgent(holo.LibP2P, holo.AgentIdentity(id), holo.MakeTestSeed(id))
	if err != nil {
		return
	}
	var hash string
	_, hash, err = agent.NodeID()
	if err != nil {
		return
	}
	pairs["%"+role+"_str%"] = id
	pairs["%"+role+"_key%"] = hash
	return
}

func saveBridgeAppsToTmpFile(bridgeAppsForTests []BridgeAppForTests) (bridgeAppsTmpfileName string, err error) {
	var bridgeApps []holo.BridgeApp
	for _, app := range bridgeAppsForTests {
		bridgeApps = append(bridgeApps, app.BridgeApp)
	}
	var tmpfile *os.File
	tmpfile, err = ioutil.TempFile("", "bridgeApp")
	if err != nil {
		return
	}
	bridgeAppsTmpfileName = tmpfile.Name()

	var data []byte
	data, err = holo.ByteEncoder(bridgeApps)
	if err != nil {
		return
	}
	_, err = tmpfile.Write(data)
	tmpfile.Close()

	return
}

func getBridgeAppsFromTmpFile(path string) (bridgeApps []holo.BridgeApp, err error) {

	data, err := holo.ReadFile(path)
	if err != nil {
		return
	}
	err = holo.ByteDecoder(data, &bridgeApps)

	return
}
