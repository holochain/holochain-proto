// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// command line interface to running holochain applications

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	holo "github.com/holochain/holochain-proto"
	"github.com/holochain/holochain-proto/cmd"
	"github.com/urfave/cli"
)

var debug bool
var verbose bool

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "hcadmin"
	app.Usage = "holochain administration tool"
	app.Version = fmt.Sprintf("0.0.4 (holochain %s)", holo.VersionStr)

	var dumpChain, dumpDHT, json bool
	var root string
	var service *holo.Service
	var bridgeCalleeAppData, bridgeCallerAppData string
	var start int

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "debugging output",
			Destination: &debug,
		},
		cli.StringFlag{
			Name:        "path",
			Usage:       "path to holochain directory (default: ~/.holochain)",
			Destination: &root,
		},
		cli.BoolFlag{
			Name:        "verbose, V",
			Usage:       "verbose output",
			Destination: &verbose,
		},
	}

	app.Commands = []cli.Command{
		{
			Name:      "init",
			Aliases:   []string{"i"},
			ArgsUsage: "agent-id",
			Usage:     "setup the holochain service",
			Action: func(c *cli.Context) error {
				agent := c.Args().First()
				if agent == "" {
					return errors.New("missing required agent-id argument to init")
				}
				_, err := holo.Init(root, holo.AgentIdentity(agent), nil)
				if err == nil {
					fmt.Println("Holochain service initialized")
					if verbose {
						fmt.Printf("    %s directory created\n", root)
						fmt.Printf("    defaults stored to %s\n", holo.SysFileName)
						fmt.Println("    key-pair generated")
						fmt.Printf("    default agent stored to %s\n", holo.AgentFileName)
					}
				}
				return err
			},
		},
		{
			Name:      "dump",
			Aliases:   []string{"d"},
			ArgsUsage: "holochain-name",
			Usage:     "display a text dump of chain and/or dht data",
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
			},
			Action: func(c *cli.Context) error {
				h, err := cmd.GetHolochain(c.Args().First(), service, "dump")
				if err != nil {
					return err
				}

				if !h.Started() {
					return errors.New("No data to dump, chain not yet initialized.")
				}
				dnaHash := h.DNAHash()
				if dumpChain {
					if json {
						dump, _ := h.Chain().JSON(start)
						fmt.Println(dump)
					} else {
						fmt.Printf("Chain for: %s\n%v", dnaHash, h.Chain().Dump(start))
					}
				}
				if dumpDHT {
					if json {
						dump, _ := h.DHT().JSON()
						fmt.Println(dump)
					} else {
						fmt.Printf("DHT for: %s\n%v", dnaHash, h.DHT())
					}
				}

				return nil
			},
		},
		{
			Name:      "join",
			Aliases:   []string{"j"},
			ArgsUsage: "path holochain-name",
			Usage:     "joins a holochain by installing an instance from an app package (or source directory) and generating genesis entries",
			Action: func(c *cli.Context) error {
				srcPath := c.Args().First()
				if srcPath == "" {
					return errors.New("join: missing required package/source path argument")
				}
				if len(c.Args()) == 1 {
					return errors.New("join: missing required holochain-name argument")
				}
				name := c.Args()[1]

				info, err := os.Stat(srcPath)
				if err != nil {
					return fmt.Errorf("join: %v", err)
				}

				// assume a regular file is a package
				if info.Mode().IsRegular() {

					dstPath := filepath.Join(root, name)
					_, err := cmd.UpackageAppPackage(service, srcPath, dstPath, name, "json")

					if err != nil {
						return fmt.Errorf("join: error unpackaging %s: %v", srcPath, err)
					}
					err = service.InitAppDir(dstPath, "json")
					if err != nil {
						return fmt.Errorf("join: error initializing the app: %v", err)
					}
				} else {
					agent, err := holo.LoadAgent(root)
					if err != nil {
						return fmt.Errorf("join: error loading agent (%s): %v", root, err)
					}
					_, err = service.Clone(srcPath, filepath.Join(root, name), agent, holo.CloneWithSameUUID, holo.InitializeDB)
					if err != nil {
						return fmt.Errorf("join: error cloning from source directory %s: %v", srcPath, err)
					}
				}
				err = genChain(service, name)
				if err != nil {
					return fmt.Errorf("join: error in chain genesis: %v", err)
				}
				if verbose {
					fmt.Printf("joined %s from %s\n", name, srcPath)
				}
				return err
			},
		},
		{
			Name:      "bridge",
			Aliases:   []string{"b"},
			ArgsUsage: "caller-chain callee-chain bridge-zome",
			Usage:     "allows caller-chain to make calls to functions in callee-chain",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "bridgeCalleeAppData",
					Usage:       "application data to pass to the bridged callee app",
					Destination: &bridgeCalleeAppData,
				},
				cli.StringFlag{
					Name:        "bridgeCallerAppData",
					Usage:       "application data to pass to the bridging caller app",
					Destination: &bridgeCallerAppData,
				},
			},
			Action: func(c *cli.Context) error {
				if len(c.Args()) != 3 {
					return errors.New("bridge: requires three arguments: from-chain to-chain bridge-zome")
				}
				callerChain := c.Args()[0]
				calleeChain := c.Args()[1]
				bridgeZome := c.Args()[2]

				hCaller, err := cmd.GetHolochain(callerChain, service, "bridge")
				if err != nil {
					return err
				}
				hCallee, err := cmd.GetHolochain(calleeChain, service, "bridge")
				if err != nil {
					return err
				}

				token, err := hCallee.AddBridgeAsCallee(hCaller.DNAHash(), bridgeCalleeAppData)
				if err != nil {
					return err
				}

				err = hCaller.AddBridgeAsCaller(bridgeZome, hCallee.DNAHash(), hCallee.Name(), token, fmt.Sprintf("http://localhost:%d", hCallee.Config.DHTPort), bridgeCallerAppData)

				if err == nil {
					if verbose {
						fmt.Printf("bridge from %s to %s\n", callerChain, calleeChain)
					}
				}
				return err
			},
		},
		{
			Name:      "status",
			Aliases:   []string{"s"},
			Usage:     "display information about installed chains",
			ArgsUsage: "[holochain-name]",
			Action: func(c *cli.Context) error {
				if service == nil {
					return cmd.ErrServiceUninitialized
				}
				if len(c.Args()) == 0 {
					fmt.Println(service.ListChains())
				} else if len(c.Args()) == 1 {
					h, err := cmd.GetHolochain(c.Args().First(), service, "status")
					if err != nil {
						return err
					}
					//					dna := h.Nucleus().DNA()
					fmt.Printf("Status of %s\n", h.Name())
					fmt.Printf("DNA Hash: %v\n", h.DNAHash())
					fmt.Printf("ID Hash: %s\n", h.NodeIDStr())
				} else {
					return errors.New("status: expected 0 or 1 argument")
				}
				return nil
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		if debug {
			os.Setenv("HCLOG_APP_ENABLE", "1")
		}
		if verbose {
			fmt.Printf("hcadmin version %s \n", app.Version)
		}
		var err error
		root, err = cmd.GetHolochainRoot(root)
		if err != nil {
			return err
		}
		service, err = cmd.GetService(root)
		if err != nil {
			if err == cmd.ErrServiceUninitialized {
				return nil // no err because service value will get tested
			}
			return err
		}
		return nil
	}

	app.Action = func(c *cli.Context) error {
		if service == nil {
			cli.ShowAppHelp(c)
		} else {
			fmt.Println(service.ListChains())
		}
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

func genChain(service *holo.Service, name string) error {
	h, err := service.GenChain(name)
	if err != nil {
		return err
	}
	if verbose {
		fmt.Printf("Genesis entries added and DNA hashed for new holochain with ID: %s\n", h.DNAHash().String())
	}
	return nil
}
