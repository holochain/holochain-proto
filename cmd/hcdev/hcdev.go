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
	"os"
	"os/user"
	"path"
	"path/filepath"
	"time"
)

const (
	defaultPort = "4141"
)

var debug, appInitialized bool
var rootPath, devPath, name string

var mutableContext map[string]string
var lastRunContext *cli.Context

func setupApp() (app *cli.App) {
  mutableContext = make(map[string]string)

	app = cli.NewApp()
	app.Name = "hcdev"
	app.Usage = "holochain dev command line tool"
	app.Version = fmt.Sprintf("0.0.1 (holochain %s)", holo.VersionStr)

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
	}

	var interactive, dumpChain, dumpDHT bool
	var clonePath, scaffoldPath string
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
			},
			ArgsUsage: "<name>",
			Action: func(c *cli.Context) error {
				if appInitialized {
					return errors.New("current directory is an initialized app, apps shouldn't be nested")
				}

				args := c.Args()
				if len(args) != 1 {
					return errors.New("init: expecting app name as single argument")
				}

				if (interactive && clonePath != "") || (interactive && scaffoldPath != "") || (clonePath != "" && scaffoldPath != "") {
					return errors.New("options are mutually exclusive, please choose just one.")
				}
				name := args[0]
				devPath = filepath.Join(devPath, name)
        if clonePath != "" {
					// build the app by cloning from another app
					info, err := os.Stat(clonePath)
					if err != nil {
						return err
					}
					if !info.Mode().IsDir() {
						return errors.New("expecting a directory to clone from")
					}

					// TODO this is the bogus dev agent, really it should probably be someone else
					agent, err := holo.LoadAgent(rootPath)
					if err != nil {
						return err
					}

					err = service.Clone(clonePath, devPath, agent, true)
					if err != nil {
						return err
					}

					fmt.Printf("cloning %s from %s\n", name, clonePath)
				} else if scaffoldPath != "" {
					// build the app from the scaffold
					info, err := os.Stat(scaffoldPath)
					if err != nil {
						return err
					}
					if !info.Mode().IsRegular() {
						return errors.New("expecting a scaffold file")
					}

					sf, err := os.Open(scaffoldPath)
					if err != nil {
						return err
					}
					defer sf.Close()

					dna, err := holo.LoadDNAScaffold(sf)
					if err != nil {
						return err
					}

					err = cmd.MakeDirs(devPath)
					if err != nil {
						return err
					}

					err = service.SaveDNAFile(devPath, dna, "json", false)
					if err != nil {
						return err
					}
					fmt.Printf("initialized %s from scaffold:%s\n", devPath, scaffoldPath)

				} else if cmd.IsFile(filepath.Join(devPath, "dna", "dna.json")) {
          cmd.OsExecPipes(cmd.GolangHolochainDir("bin", "holochain.app.init.interactive"))
          ranScript = true
        } else {

          // build empty app template
					err := cmd.MakeDirs(devPath)
					if err != nil {
						return err
					}
					scaffold := bytes.NewBuffer([]byte(holo.BasicTemplateScaffold))
					dna, err := holo.LoadDNAScaffold(scaffold)
					if err != nil {
						return err
					}
					dna.NewUUID()
					err = service.SaveDNAFile(devPath, dna, "json", false)
					if err != nil {
						return err
					}
					fmt.Printf("initialized empty application to %s with new UUID:%v\n", devPath, dna.UUID)
				}

				err := os.Chdir(devPath)
				if err != nil {
					return err
				}

				// finish by creating the .hc directory
				// terminates go process
				if ! ranScript {
          cmd.ExecBinScript("holochain.app.init", name, name)
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

				// terminates go process
				cmd.ExecBinScript("holochain.app.testScenario", args[0])
				return nil
			},
		},
    {
      Name:      "goScenario",
      Aliases:   []string{"S"},
      Usage:     "run a scenario test",
      ArgsUsage: "scenario-name",
      Action: func(c *cli.Context) error {
        mutableContext["command"] = "goScenario"

        if !appInitialized {
          return errors.New("please initialize this app with 'hcdev init'")
        }

        args := c.Args()
        if len(args) != 1 {
          return errors.New("missing scenario name argument")
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

				h, err := getHolochain(c, service)
				if err != nil {
					return err
				}
				h, err = service.GenChain(name)
				if err != nil {
					return err
				}

				var port string
				if len(c.Args()) == 0 {
					port = defaultPort
				} else {
					port = c.Args()[0]
				}
				fmt.Printf("Serving holochain with DNA hash:%v on port:%s\n", h.DNAHash(), port)

				err = h.Activate()
				if err != nil {
					return err
				}
				//				go h.DHT().HandleChangeReqs()
				go h.DHT().HandleGossipWiths()
				go h.DHT().Gossip(2 * time.Second)
				ui.NewWebServer(h, port).Start()
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
		lastRunContext = c

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
	err = service.Clone(devPath, filepath.Join(rootPath, name), agent, false)
	if err != nil {
		return
	}
	h, err = service.Load(name)
	if err != nil {
		return
	}
	return
}

func GetLastRunContext () (map[string]string, *cli.Context) {
  return mutableContext, lastRunContext
}