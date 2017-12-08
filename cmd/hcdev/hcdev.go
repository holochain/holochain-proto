// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// command line interface to developing and testing holochain applications

package main

import (
	"bytes"
	"errors"
	"fmt"
	holo "github.com/metacurrency/holochain"
	. "github.com/metacurrency/holochain/apptest"
	"github.com/metacurrency/holochain/cmd"
	hash "github.com/metacurrency/holochain/hash"
	"github.com/metacurrency/holochain/ui"
	"github.com/urfave/cli"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
	// fsnotify	"github.com/fsnotify/fsnotify"
	//spew "github.com/davecgh/go-spew/spew"
)

const (
	defaultPort        = "4141"
	bridgeFromPort     = "21111"
	bridgeToPort       = "21112"
	scenarioStartDelay = 1
)

var debug, appInitialized, verbose, keepalive bool
var rootPath, devPath, bridgeToPath, bridgeToName, bridgeFromPath, bridgeFromName, name string
var bridgeFromH, bridgeToH *holo.Holochain
var bridgeFromAppData, bridgeToAppData string
var scenarioConfig *holo.TestConfig

// flags for holochain config generation
var port, logPrefix, bootstrapServer string
var mdns bool
var nonatupnp bool

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

	// clear these values so we can call this multiple time for testing
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
	app.Version = fmt.Sprintf("0.0.3 (holochain %s)", holo.VersionStr)

	var service *holo.Service
	var serverID, agentID, identity string

	var scenarioTmpDir = "hcdev_scenario_test_nodes_" + sysUser.Username

	var dumpScenario string
	var dumpTest bool

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
			Name:        "port",
			Usage:       "port on which to run the test/scenario instance",
			Destination: &port,
		},
		cli.BoolFlag{
			Name:        "mdns",
			Usage:       "whether to use mdns for local peer discovery",
			Destination: &mdns,
		},
		cli.BoolFlag{
			Name:        "no-nat-upnp",
			Usage:       "whether to stop hcdev from creating a port mapping through NAT via UPnP",
			Destination: &nonatupnp,
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
			Name:        "bridgeTo",
			Usage:       "path to dev directory of app to bridge to",
			Destination: &bridgeToPath,
		},
		cli.StringFlag{
			Name:        "bridgeFrom",
			Usage:       "path to dev directory of app to bridge from",
			Destination: &bridgeFromPath,
		},
		cli.StringFlag{
			Name:        "bridgeToAppData",
			Usage:       "application data to pass to the bridged to app",
			Destination: &bridgeToAppData,
		},
		cli.StringFlag{
			Name:        "bridgeFromAppData",
			Usage:       "application data to pass to the bridging from app",
			Destination: &bridgeFromAppData,
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

	var interactive, dumpChain, dumpDHT, initTest, fromDevelop bool
	var clonePath, appPackagePath, cloneExample, outputDir, fromBranch string
	app.Commands = []cli.Command{
		{
			Name:    "init",
			Aliases: []string{"i"},
			Usage:   "initialize a holochain app directory: interactively, from a appPackage file or clone from another app",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "interactive",
					Usage:       "interactive initialization",
					Destination: &interactive,
				},
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
				if interactive {
					flags += 1
				}
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
				devPath = filepath.Join(devPath, name)

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
					command := exec.Command("git", "clone", fmt.Sprintf("git://github.com/Holochain/%s.git", cloneExample))
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
					fmt.Printf("cloning %s from github.com/Holochain/%s\n", name, cloneExample)
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
			},
			Action: func(c *cli.Context) error {
				holo.Debug("test: start")

				var err error
				if err = appCheck(devPath); err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				var h *holo.Holochain
				var bridgeApps []holo.BridgeApp
				h, bridgeApps, err = getHolochain(c, service, identity)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				holo.Debug("test: initialised holochain\n")

				args := c.Args()
				var errs []error

				if len(args) == 2 {

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
					} else {

						// if there isn't a clone then we can do role substitutions
						var roles []string
						roles, err = holo.GetTestScenarioRoles(h, scenario)
						if err != nil {
							return cmd.MakeErrFromErr(c, err)
						}
						host := getHostName(serverID)
						for _, role := range roles {
							var id, hash string
							id = role + "@" + host
							agent, err := holo.NewAgent(holo.LibP2P, holo.AgentIdentity(id), holo.MakeTestSeed(id))
							if err != nil {
								return cmd.MakeErrFromErr(c, err)
							}
							_, hash, err = agent.NodeID()
							if err != nil {
								return cmd.MakeErrFromErr(c, err)
							}
							pairs["%"+role+"_str%"] = id
							pairs["%"+role+"_key%"] = hash
						}
					}
					err, errs = TestScenario(h, scenario, role, pairs)
					if err != nil {
						return cmd.MakeErrFromErr(c, err)
					}
					//holo.Debugf("testScenario: h: %v\n", spew.Sdump(h))

				} else if len(args) == 1 {
					errs = TestOne(h, args[0], bridgeApps)
				} else if len(args) == 0 {
					errs = Test(h, bridgeApps)
				} else {
					return cmd.MakeErr(c, "expected 0 args (run all stand-alone tests), 1 arg (a single stand-alone test) or 2 args (scenario and role)")
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
			},
			Action: func(c *cli.Context) error {
				mutableContext.str["command"] = "scenario"

				if err := appCheck(devPath); err != nil {
					return err
				}

				if bridgeFromPath != "" || bridgeToPath != "" {
					return cmd.MakeErr(c, "bridging not supported in scenario tests yet")
				}
				args := c.Args()
				if len(args) != 1 {
					return cmd.MakeErr(c, "missing scenario name argument")
				}
				scenarioName := args[0]

				// get the holochain from the source that we are supposed to be testing
				h, _, err := getHolochain(c, service, identity)
				if err != nil {
					return cmd.MakeErrFromErr(c, err)
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

				scenarioConfig, err = holo.LoadTestConfig(filepath.Join(h.TestPath(), scenarioName))
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

					// HOLOCHAINCONFIG_PORT       = FindSomeAvailablePort
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

						var nonat string
						if bootstrapServer == "_" {
							nonat = "true"
						} else {
							nonat = "false"
						}

						testCommand := exec.Command(
							"hcdev",
							"-path="+devPath,
							"-execpath="+filepath.Join(rootExecDir, roleName),
							"-port="+strconv.Itoa(freePort),
							fmt.Sprintf("-mdns=%v", mdns),
							"-no-nat-upnp="+nonat,
							"-logPrefix="+logPrefix,
							"-serverID="+serverID,
							"-agentID="+agentID,
							fmt.Sprintf("-bootstrapServer=%v", bootstrapServer),
							fmt.Sprintf("-keepalive=%v", keepalive),
							"test",
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
				return nil
			},
		},
		{
			Name:      "web",
			Aliases:   []string{"serve", "w"},
			ArgsUsage: "[port]",
			Usage:     fmt.Sprintf("serve a chain to the web on localhost:<port> (defaults to %s)", defaultPort),
			Action: func(c *cli.Context) error {
				if err := appCheck(devPath); err != nil {
					return cmd.MakeErrFromErr(c, err)
				}

				h, bridgeApps, err := getHolochain(c, service, agentID)
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
					port = defaultPort
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
				h, _, err := getHolochain(c, service, identity)
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
					Name:        "test",
					Destination: &dumpTest,
				},
				cli.StringFlag{
					Name:        "scenario",
					Destination: &dumpScenario,
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
					fmt.Printf("Chain for: %s\n%v", dnaHash, h.Chain())
				}
				if dumpDHT {
					fmt.Printf("DHT for: %s\n%v", dnaHash, h.DHT().String())
				}

				return nil
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		holo.IsDevMode = true
		lastRunContext = c

		var err error

		if port != "" {
			err = os.Setenv("HOLOCHAINCONFIG_PORT", port)
			if err != nil {
				return err
			}
		}
		if mdns != false {
			err = os.Setenv("HOLOCHAINCONFIG_ENABLEMDNS", "true")
			if err != nil {
				return err
			}
		}
		if nonatupnp == false {
			err = os.Setenv("HOLOCHAINCONFIG_ENABLENATUPNP", "true")
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
	if verbose {
		fmt.Printf("hcdev complete!\n")
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func getHolochain(c *cli.Context, service *holo.Service, identity string) (h *holo.Holochain, bridgeApps []holo.BridgeApp, err error) {
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
		agent.SetIdentity(holo.AgentIdentity(identity))
		agent.GenKeys(holo.MakeTestSeed(identity))
	}

	bridgeApps = make([]holo.BridgeApp, 0)

	if bridgeToPath != "" && bridgeFromPath != "" {
		if bridgeFromAppData != "" || bridgeToAppData != "" {
			// TODO The reason for this is that we have no way of collecting the
			// separate to&from app data that would be needed for both apps.
			err = errors.New("hcdev currently only supports one bridge app if passing in appData")
			return
		}
	}

	if bridgeToPath != "" {
		bridgeToH, err = setupBridgeApp(service, h, agent, bridgeToPath, holo.BridgeFrom)
		if err != nil {
			return
		}
		bridgeApps = append(bridgeApps,
			holo.BridgeApp{
				H:    bridgeToH,
				Side: holo.BridgeTo,
				BridgeGenesisDataFrom: bridgeFromAppData,
				BridgeGenesisDataTo:   bridgeToAppData,
				Port:                  bridgeToPort})
	}
	if bridgeFromPath != "" {
		bridgeFromH, err = setupBridgeApp(service, h, agent, bridgeFromPath, holo.BridgeTo)
		if err != nil {
			return
		}
		bridgeApps = append(bridgeApps,
			holo.BridgeApp{
				H:    bridgeFromH,
				Side: holo.BridgeFrom,
				BridgeGenesisDataFrom: bridgeFromAppData,
				BridgeGenesisDataTo:   bridgeToAppData,
				Port:                  bridgeFromPort})
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

// setupBridgeApp clones the bridge app from source and loads it in preparation for actual bridging
func setupBridgeApp(service *holo.Service, h *holo.Holochain, agent holo.Agent, path string, side int) (bridgeH *holo.Holochain, err error) {

	bridgeName := filepath.Base(path)

	os.Setenv("HOLOCHAINCONFIG_ENABLEMDNS", "true")
	os.Setenv("HOLOCHAINCONFIG_BOOTSTRAP", "_")
	os.Setenv("HCLOG_PREFIX", bridgeName+":")
	if side == holo.BridgeFrom {
		os.Setenv("HOLOCHAINCONFIG_PORT", "9991")
	} else {
		os.Setenv("HOLOCHAINCONFIG_PORT", "9992")
	}
	fmt.Printf("Copying bridge chain %s to: %s\n", bridgeName, rootPath)
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

	// set the dna for use by the dev BridgeTo resolver
	var DNAHash hash.Hash
	DNAHash, err = holo.DNAHashofUngenedChain(bridgeH)
	if err != nil {
		return
	}
	holo.DevDNAResolveMap = make(map[string]string)
	holo.DevDNAResolveMap[bridgeName] = DNAHash.String()

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
