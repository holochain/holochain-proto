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
	"github.com/metacurrency/holochain/cmd"
	"github.com/metacurrency/holochain/ui"
	"github.com/urfave/cli"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"time"
)

const (
	defaultPort = "4141"
)

var debug, appInitialized bool
var rootPath, devPath, bridgeToPath, bridgeToName, bridgeFromPath, bridgeFromName, name string
var bridgeFromH, bridgeToH *holo.Holochain

// TODO: move these into cmd module
func makeErr(prefix string, text string, code int) error {
	errText := fmt.Sprintf("%s: %s", prefix, text)
	fmt.Printf("CLI Error: %s\n", errText)
	return cli.NewExitError(errText, 1)
}

func makeErrFromError(prefix string, err error, code int) error {
	return makeErr(prefix, err.Error(), code)
}

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "hcdev"
	app.Usage = "holochain dev command line tool"
	app.Version = fmt.Sprintf("0.0.2 (holochain %s)", holo.VersionStr)

	var service *holo.Service

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "debugging output",
			Destination: &debug,
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
			Name:        "bridgeTo",
			Usage:       "path to dev directory of app to bridge to",
			Destination: &bridgeToPath,
		},
		cli.StringFlag{
			Name:        "bridgeFrom",
			Usage:       "path to dev directory of app to bridge from",
			Destination: &bridgeFromPath,
		},
	}

	var interactive, dumpChain, dumpDHT, initTest bool
	var clonePath, scaffoldPath, cloneExample string
	var ranScript bool
	app.Commands = []cli.Command{
		{
			Name:    "init",
			Aliases: []string{"i"},
			Usage:   "initialize a holochain app directory: interactively, from a scaffold file or clone from another app",
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
					Usage:       "path to directory from which to clone the app",
					Destination: &clonePath,
				},
				cli.StringFlag{
					Name:        "scaffold",
					Usage:       "path to a scaffold file from which to initialize the app",
					Destination: &scaffoldPath,
				},
				cli.StringFlag{
					Name:        "cloneExample",
					Usage:       "example from github.com/holochain to clone from",
					Destination: &cloneExample,
				},
			},
			ArgsUsage: "<name>",
			Action: func(c *cli.Context) error {
				if appInitialized {
					return makeErr("init", "current directory is an initialized app, apps shouldn't be nested", 1)
				}

				args := c.Args()
				if len(args) != 1 {
					return makeErr("init", "expecting app name as single argument", 1)
				}
				flags := 0
				if interactive {
					flags += 1
				}
				if clonePath != "" {
					flags += 1
				}
				if scaffoldPath != "" {
					flags += 1
				}
				if initTest {
					flags += 1
				}
				if flags > 1 {
					return makeErr("init", " options are mutually exclusive, please choose just one.", 1)
				}
				name := args[0]
				devPath = filepath.Join(devPath, name)
				if initTest {
					fmt.Printf("initializing test app as %s\n", name)
					format := "json"
					if len(c.Args()) == 2 {
						format = c.Args()[1]
						if !(format == "json" || format == "yaml" || format == "toml") {
							return makeErr("init", "format must be one of yaml,toml,json", 1)

						}
					}
					_, err := service.GenDev(devPath, "json", holo.SkipInitializeDB)
					if err != nil {
						return makeErrFromError("init", err, 1)
					}
				} else if clonePath != "" {
					// build the app by cloning from another app
					info, err := os.Stat(clonePath)
					if err != nil {
						dir, _ := cmd.GetCurrentDirectory()
						return makeErrFromError(fmt.Sprintf("ClonePath:%s/'%s'", dir, clonePath), err, 1)
					}

					if !info.Mode().IsDir() {
						return makeErr("init", "-clone flag expects a directory to clone from", 1)
					}
					fmt.Printf("cloning %s from %s\n", name, clonePath)
					err = doClone(service, clonePath, devPath)
					if err != nil {
						return makeErrFromError("init", err, 1)
					}
				} else if cloneExample != "" {
					tmpCopyDir, err := ioutil.TempDir("", fmt.Sprintf("holochain.example.%s", cloneExample))
					if err != nil {
						return makeErrFromError("init", err, 1)
					}
					defer os.RemoveAll(tmpCopyDir)
					err = os.Chdir(tmpCopyDir)
					if err != nil {
						return makeErrFromError("init", err, 1)
					}
					cmd := exec.Command("git", "clone", fmt.Sprintf("git://github.com/Holochain/%s.git", cloneExample))
					out, err := cmd.CombinedOutput()
					fmt.Printf("git: %s\n", string(out))
					if err != nil {
						return makeErrFromError("init", err, 1)
					}
					clonePath := filepath.Join(tmpCopyDir, cloneExample)
					fmt.Printf("cloning %s from github.com/Holochain/%s\n", name, cloneExample)
					err = doClone(service, clonePath, devPath)

				} else if scaffoldPath != "" {
					// build the app from the scaffold
					info, err := os.Stat(scaffoldPath)
					if err != nil {
						return makeErrFromError("init", err, 1)
					}
					if !info.Mode().IsRegular() {
						return makeErr("init", "-scaffold flag expectings a scaffold file", 1)
					}

					sf, err := os.Open(scaffoldPath)
					if err != nil {
						return makeErrFromError("init", err, 1)
					}
					defer sf.Close()

					_, err = service.SaveScaffold(sf, devPath, false)
					if err != nil {
						return makeErrFromError("init", err, 1)
					}

					fmt.Printf("initialized %s from scaffold:%s\n", devPath, scaffoldPath)

				} else if cmd.IsFile(filepath.Join(devPath, "dna", "dna.json")) {
					cmd.OsExecPipes(cmd.GolangHolochainDir("bin", "holochain.app.init.interactive"))
					ranScript = true
				} else {

					// build empty app template
					err := holo.MakeDirs(devPath)
					if err != nil {
						return makeErrFromError("init", err, 1)
					}
					scaffoldReader := bytes.NewBuffer([]byte(holo.BasicTemplateScaffold))

					var scaffold *holo.Scaffold
					scaffold, err = service.SaveScaffold(scaffoldReader, devPath, true)
					if err != nil {
						return makeErrFromError("init", err, 1)
					}
					fmt.Printf("initialized empty application to %s with new UUID:%v\n", devPath, scaffold.DNA.UUID)
				}

				err := os.Chdir(devPath)
				if err != nil {
					return makeErrFromError("", err, 1)
				}

				// finish by creating the .hc directory
				// terminates go process
				if !ranScript {
					cmd.OsExecPipes("holochain.app.init", name)
				}
				return nil
			},
		},
		{
			Name:      "test",
			Aliases:   []string{"t"},
			ArgsUsage: "no args run's all stand-alone | [test file prefix] | [scenario] [role]",
			Usage:     "run chain's stand-alone or scenario tests",
			Action: func(c *cli.Context) error {
				var err error
				var h *holo.Holochain
				h, err = getHolochain(c, service)
				if err != nil {
					return err
				}

				err = activateBridgedApps(service)
				if err != nil {
					return err
				}

				args := c.Args()
				var errs []error

				if len(args) == 2 {
					dir := filepath.Join(h.TestPath(), args[0])
					role := args[1]

					err, errs = h.TestScenario(dir, role)
					if err != nil {
						return err
					}
				} else if len(args) == 1 {
					errs = h.TestOne(args[0])
				} else if len(args) == 0 {
					errs = h.Test()
				} else {
					return errors.New("test: expected 0 args (run all stand-alone tests), 1 arg (a single stand-alone test) or 2 args (scenario and role)")
				}

				var s string
				for _, e := range errs {
					s += e.Error()
				}
				if s != "" {
					return errors.New(s)
				}
				return nil
			},
		},
		{
			Name:      "scenario",
			Aliases:   []string{"s"},
			Usage:     "run a scenario test",
			ArgsUsage: "scenario-name",
			Action: func(c *cli.Context) error {
				if !appInitialized {
					return errors.New("please initialize this app with 'hcdev init'")
				}

				args := c.Args()
				if len(args) != 1 {
					return errors.New("missing scenario name argument")
				}

				err := activateBridgedApps(service)
				if err != nil {
					return err
				}
				// terminates go process
				cmd.ExecBinScript("holochain.app.testScenario", args[0])
				return nil
			},
		},
		{
			Name:      "web",
			Aliases:   []string{"serve", "w"},
			ArgsUsage: "[port]",
			Usage:     fmt.Sprintf("serve a chain to the web on localhost:<port> (defaults to %s)", defaultPort),
			Action: func(c *cli.Context) error {

				h, err := getHolochain(c, service)
				if err != nil {
					return err
				}
				h.Close()
				h, err = service.GenChain(name)
				if err != nil {
					return err
				}
				err = activateBridgedApps(service)
				if err != nil {
					return err
				}

				var port string
				if len(c.Args()) == 0 {
					port = defaultPort
				} else {
					port = c.Args()[0]
				}

				err = activate(h, port)
				return err
			},
		},
		{
			Name:      "dump",
			Aliases:   []string{"d"},
			ArgsUsage: "holochain-name",
			Usage:     "display a text dump of a chain after last test",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "chain",
					Destination: &dumpChain,
				},
				cli.BoolFlag{
					Name:        "dht",
					Destination: &dumpDHT,
				},
			},
			Action: func(c *cli.Context) error {

				if !dumpChain && !dumpDHT {
					dumpChain = true
				}
				h, err := service.Load(name)
				if err != nil {
					return err
				}

				if !h.Started() {
					return errors.New("No data to dump, chain not yet initialized.")
				}

				dnaHash := h.DNAHash()
				if dumpChain {
					fmt.Printf("Chain for: %s\n%v", dnaHash, h.Chain())
				}
				if dumpDHT {
					fmt.Printf("DHT for: %s\n%v", dnaHash, h.DHT())
				}

				return nil
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		if debug {
			os.Setenv("DEBUG", "1")
		}
		holo.InitializeHolochain()

		var err error
		if devPath == "" {
			devPath, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		name = path.Base(devPath)

		if cmd.IsAppDir(devPath) == nil {
			appInitialized = true
		}

		if rootPath == "" {
			rootPath = os.Getenv("HOLOPATHDEV")
			if rootPath == "" {
				u, err := user.Current()
				if err != nil {
					return err
				}
				userPath := u.HomeDir
				rootPath = filepath.Join(userPath, holo.DefaultDirectoryName+"dev")
			}
		}
		if !holo.IsInitialized(rootPath) {
			u, err := user.Current()
			var agent string
			if err == nil {
				var host string
				host, err = os.Hostname()
				if err == nil {
					agent = u.Username + "@" + host
				}
			}

			if err != nil {
				agent = "test@example.com"
			}
			service, err = holo.Init(rootPath, holo.AgentIdentity(agent))
			if err != nil {
				return err
			}
			fmt.Println("Holochain dev service initialized:")
			fmt.Printf("    %s directory created\n", rootPath)
			fmt.Printf("    defaults stored to %s\n", holo.SysFileName)
			fmt.Println("    key-pair generated")
			fmt.Printf("    using %s as default agent (stored to %s)\n", agent, holo.AgentFileName)

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
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func getHolochain(c *cli.Context, service *holo.Service) (h *holo.Holochain, err error) {
	fmt.Printf("Copying chain to: %s\n", rootPath)
	err = os.RemoveAll(filepath.Join(rootPath, name))
	if err != nil {
		return
	}
	var agent holo.Agent
	agent, err = holo.LoadAgent(rootPath)
	if err != nil {
		return
	}
	err = service.Clone(devPath, filepath.Join(rootPath, name), agent, holo.CloneWithSameUUID, holo.InitializeDB)
	if err != nil {
		return
	}

	h, err = service.Load(name)
	if err != nil {
		return
	}
	if bridgeToPath != "" {
		bridgeToH, err = bridge(service, h, agent, bridgeToPath, true)
		if err != nil {
			return
		}
	}
	if bridgeFromPath != "" {
		bridgeFromH, err = bridge(service, h, agent, bridgeFromPath, false)
		if err != nil {
			return
		}
	}
	return
}

func bridge(service *holo.Service, h *holo.Holochain, agent holo.Agent, path string, isFrom bool) (bridgeH *holo.Holochain, err error) {

	bridgeName := filepath.Base(path)

	os.Setenv("HOLOCHAINCONFIG_ENABLEMDNS", "true")
	os.Setenv("HOLOCHAINCONFIG_BOOTSTRAP", "_")
	os.Setenv("HOLOCHAINCONFIG_LOGPREFIX", bridgeName+":")
	if isFrom {
		os.Setenv("HOLOCHAINCONFIG_PORT", "9991")
	} else {
		os.Setenv("HOLOCHAINCONFIG_PORT", "9992")
	}
	var hFrom, hTo *holo.Holochain
	fmt.Printf("Copying bridge chain %s to: %s\n", bridgeName, rootPath)
	err = os.RemoveAll(filepath.Join(rootPath, bridgeName))
	if err != nil {
		return
	}
	err = service.Clone(path, filepath.Join(rootPath, bridgeName), agent, holo.CloneWithSameUUID, holo.InitializeDB)
	if err != nil {
		return
	}
	bridgeH, err = service.Load(bridgeName)
	if err != nil {
		return
	}

	if isFrom {
		bridgeToName = bridgeName
		hFrom = bridgeH
		hTo = h
	} else {
		bridgeFromName = bridgeName
		hFrom = h
		hTo = bridgeH
	}

	var token string
	token, err = hTo.NewBridge()
	if err != nil {
		return
	}

	err = hFrom.AddBridge(hTo.DNAHash(), token, fmt.Sprintf("http://localhost:%d", hTo.Config().Port))
	if err != nil {
		return
	}
	return
}

func activateBridgedApp(s *holo.Service, h *holo.Holochain, name string, port string) (err error) {
	h, err = s.GenChain(name)
	if err != nil {
		return
	}
	go activate(h, port)
	return
}

func activateBridgedApps(s *holo.Service) (err error) {
	if bridgeFromH != nil {
		err = activateBridgedApp(s, bridgeFromH, bridgeFromName, "12346")
		if err != nil {
			return
		}
	}
	if bridgeToH != nil {
		err = activateBridgedApp(s, bridgeToH, bridgeToName, "12345")
		if err != nil {
			return
		}
	}
	return
}

func activate(h *holo.Holochain, port string) (err error) {
	fmt.Printf("Serving holochain with DNA hash:%v on port:%s\n", h.DNAHash(), port)
	err = h.Activate()
	if err != nil {
		return
	}
	//				go h.DHT().HandleChangeReqs()
	go h.DHT().HandleGossipWiths()
	go h.DHT().Gossip(2 * time.Second)
	ui.NewWebServer(h, port).Start()
	return
}

func doClone(s *holo.Service, clonePath, devPath string) (err error) {

	// TODO this is the bogus dev agent, really it should probably be someone else
	agent, err := holo.LoadAgent(rootPath)
	if err != nil {
		return
	}

	err = s.Clone(clonePath, devPath, agent, holo.CloneWithSameUUID, holo.SkipInitializeDB)
	if err != nil {
		return
	}
	return
}
