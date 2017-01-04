package main

import (
	"fmt"
	"os"

	holo "github.com/metacurrency/holochain"
	"github.com/urfave/cli"
	"github.com/BurntSushi/toml"

	//"github.com/google/uuid"
)


func main() {
	app := cli.NewApp()
	app.Name = "hc"
	app.Usage = "holochain command line interface"
	var verbose,initialized bool

	app.Flags = []cli.Flag {
		cli.BoolFlag{
			Name: "verbose",
			Usage: "verbose output",
			Destination: &verbose,
		},
	}


	app.Commands = []cli.Command{
		{
			Name:    "init",
			Aliases: []string{"i"},
			Usage:   "initialize a new holochain in the current directory",
			Action:  func(c *cli.Context) error {
				h,err := holo.Init()
				if err == nil {
					if (verbose) {
						fmt.Printf("initialized new holochain with id: %v\n",h.Id);
					}
				}
				return err
			},
		},
		{
			Name:    "link",
			Aliases: []string{"l"},
			Usage:   "add a link onto the chain",
			Action:  func(c *cli.Context) error {
				fmt.Println("link unimplemented (args:", c.Args().First(),")")
				return nil
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		if (verbose) {
			fmt.Println("Holochain version ",holo.Version)
		}
		initialized = holo.IsInitialized()
		return nil
	}

	app.Action = func(c *cli.Context) error {
		if (!initialized) {
			cli.ShowAppHelp(c)
		} else {
			var config holo.Config
			if _, err := toml.DecodeFile(holo.ConfigPath, &config); err != nil {
				fmt.Println("Error parsing config file")
				return err
			}
			fmt.Printf("current config: \n%v\n",config)
		}
		return nil
	}

	app.Run(os.Args)
}
