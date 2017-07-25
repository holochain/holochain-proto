// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// command line interface to running holochain applications

package main

import (
	"errors"
	"fmt"
	holo "github.com/metacurrency/holochain"
	"github.com/metacurrency/holochain/cmd"
	"github.com/urfave/cli"
	"os"
	"path/filepath"
)

var debug bool
var verbose bool

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "hcadmin"
	app.Usage = "holochain administration tool"
	app.Version = fmt.Sprintf("0.0.1 (holochain %s)", holo.VersionStr)

	var dumpChain, dumpDHT bool
	var root string
	var service *holo.Service

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
				_, err := holo.Init(root, holo.AgentName(agent))
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
					fmt.Printf("Chain for: %s\n%v", dnaHash, h.Chain())
				}
				if dumpDHT {
					fmt.Printf("DHT for: %s\n%v", dnaHash, h.DHT())
				}

				return nil
			},
		},
		{
			Name:      "join",
			Aliases:   []string{"j"},
			ArgsUsage: "src-path holochain-name",
			Usage:     "joins a holochain by copying an instance from a source and generating genesis blocks",
			Action: func(c *cli.Context) error {
				srcPath := c.Args().First()
				if srcPath == "" {
					return errors.New("join: missing required source path argument")
				}
				if len(c.Args()) == 1 {
					return errors.New("join: missing required holochain-name argument")
				}
				name := c.Args()[1]
				agent, err := holo.LoadAgent(root)
				if err != nil {
					return err
				}
				err = service.Clone(srcPath, filepath.Join(root, name), agent, false)
				if err == nil {
					if verbose {
						fmt.Printf("joined %s from %s\n", name, srcPath)
					}
					err = genChain(service, name)
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
					dna := h.Nucleus().DNA()
					fmt.Printf("Status of %s\n", dna.Name)
					fmt.Printf("   ---More status info here, no yet implmented---\n")
				} else {
					return errors.New("status: expected 0 or 1 argument")
				}
				return nil
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		if debug {
			os.Setenv("DEBUG", "1")
		}
		if verbose {
			fmt.Printf("hcadmin version %s \n", app.Version)
		}
		var err error
		service, err = cmd.GetService(root)
		if err != nil {
			if err == cmd.ErrServiceUninitialized {
				return nil // no err because service value will get tested
			}
			return err
		}
		root = service.Path
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
