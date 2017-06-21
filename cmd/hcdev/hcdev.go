// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// command line interface to developing and testing holochain applications

package main

import (
	"errors"
	"fmt"
	holo "github.com/metacurrency/holochain"
	"github.com/urfave/cli"
	"os"
	"os/user"
	"path"
)

const (
	defaultPort = "4141"
)

var debug bool

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "hcdev"
	app.Usage = "holochain dev command line tool"
	app.Version = fmt.Sprintf("0.0.0 (holochain %s)", holo.VersionStr)
	var rootPath, devPath, name string
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
	app.Commands = []cli.Command{
		{
			Name:    "test",
			Aliases: []string{"t"},
			Usage:   "run chain's unit or scenario tests",
			Action: func(c *cli.Context) error {
				var err error
				var h *holo.Holochain

				fmt.Printf("Copying chain to: %s\n", rootPath)
				err = os.RemoveAll(rootPath + "/" + name)
				if err != nil {
					return err
				}

				h, err = service.Clone(devPath, rootPath+"/"+name, false)
				if err != nil {
					return err
				}

				h, err = service.Load(name)
				if err != nil {
					return err
				}

				args := c.Args()
				var errs []error

				if len(args) == 2 {
					dir := h.TestPath() + "/" + args[0]
					role := args[1]

					err, errs = h.TestScenario(dir, role)
					if err != nil {
						return err
					}
				} else if len(args) != 0 {
					return errors.New("test: expected 0 args or 2 (scenario and role)")
				} else {
					errs = h.Test()
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
	}

	app.Before = func(c *cli.Context) error {
		if debug {
			os.Setenv("DEBUG", "1")
		}
		holo.Initialize()

		var err error
		if devPath == "" {
			devPath, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		name = path.Base(devPath)
		// TODO confirm devPath is actually a holochain app directory

		if rootPath == "" {
			rootPath = os.Getenv("HOLOPATH")
			if rootPath == "" {
				u, err := user.Current()
				if err != nil {
					return err
				}
				userPath := u.HomeDir
				rootPath = userPath + "/" + holo.DefaultDirectoryName + "dev"
			}
		}
		if !holo.IsInitialized(rootPath) {
			service, err = holo.Init(rootPath, holo.AgentName("test@example.com"))
			if err != nil {
				return err
			}
			fmt.Println("Holochain dev service initialized:")
			fmt.Printf("    %s directory created\n", rootPath)
			fmt.Printf("    defaults stored to %s\n", holo.SysFileName)
			fmt.Println("    key-pair generated")
			fmt.Printf("    default agent stored to %s\n", holo.AgentFileName)

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
