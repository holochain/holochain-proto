// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// command line interface to running holochain applications

package main

import (
	"fmt"
	holo "github.com/metacurrency/holochain"
	"github.com/metacurrency/holochain/cmd"
	"github.com/metacurrency/holochain/ui"
	"github.com/urfave/cli"
	"os"
)

const (
	defaultPort = "3141"
)

var debug bool
var verbose bool
var nonatupnp bool

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "hcd"
	app.Usage = fmt.Sprintf("serve a chain to the web on localhost:<port> (defaults to %s)", defaultPort)
	app.ArgsUsage = "holochain-name [port]"

	app.Version = fmt.Sprintf("0.0.3 (holochain %s)", holo.VersionStr)

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

	app.Before = func(c *cli.Context) error {
		// for hcd the -debug flag enables the app level debugging
		if debug {
			os.Setenv("HCLOG_APP_ENABLE", "1")
		}
		if verbose {
			fmt.Printf("hcd version %s \n", app.Version)
		}
		var err error
		root, err = cmd.GetHolochainRoot(root)
		if err != nil {
			return err
		}
		service, err = cmd.GetService(root)
		if err != nil {
			return err
		}
		return nil
	}

	app.Action = func(c *cli.Context) error {
		args := len(c.Args())
		if args == 1 || args == 2 {
			h, err := cmd.GetHolochain(c.Args().First(), service, "serve")
			if err != nil {
				return err
			}
			if !h.Started() {
				return fmt.Errorf("Can't serve an un-started chain!\n")
			}

			var port string
			if len(c.Args()) == 1 {
				port = defaultPort
			} else {
				port = c.Args()[1]
			}
			err = h.Activate()
			if err != nil {
				return err
			}

			h.StartBackgroundTasks()

			fmt.Printf("Serving holochain with DNA hash:%v on port %s\n", h.DNAHash(), port)

			ws := ui.NewWebServer(h, port)
			ws.Start()
			ws.Wait()
			return err
		} else if args == 0 {
			fmt.Println(service.ListChains())
		} else {
			return fmt.Errorf("Expected single holochain-name argument with optional port argument.\n")
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
