package main

import (
	"fmt"
	"os"
	"errors"

	holo "github.com/metacurrency/holochain"
	"github.com/urfave/cli"

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
			Name:    "gen",
			Aliases: []string{"g"},
			Subcommands: []cli.Command{
				{
					Name:  "dna",
					Usage: "generate a default configuration files",
					Action: func(c *cli.Context) error {
						h,err := holo.GenDNA()
						if err == nil {
							if (verbose) {
								fmt.Printf("created .holochain/config with new id: %v\n",h.Id);
							}
						}
						return err
					},
				},
				{
					Name:  "key",
					Usage: "generate a key pair for entry signing",
					Action: func(c *cli.Context) error {
						return errors.New("not implemented")
					},
				},
				{
					Name:  "chain",
					Usage: "generate the genesis blocks from the configuration and keys",
					Action: func(c *cli.Context) error {
						return holo.GenChain()
					},
				},
			},
		},
		{
			Name:    "link",
			Aliases: []string{"l"},
			Usage:   "add an entry onto the chain",
			Action:  func(c *cli.Context) error {
				return errors.New("not implemented")
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

			h,err := holo.Load()
			if  err != nil {
				return err
			}
			fmt.Printf("current config: \n%v\n",h)
		}
		return nil
	}

	app.Run(os.Args)
}
