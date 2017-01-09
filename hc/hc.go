package main

import (
	"fmt"
	"os"
	"os/user"
	"errors"

	holo "github.com/metacurrency/holochain"
	"github.com/urfave/cli"

	//"github.com/google/uuid"
)


func main() {
	app := cli.NewApp()
	app.Name = "hc"
	app.Usage = "holochain peer command line interface"
	app.Version = "0.0.1"
	var verbose,initialized bool
	var root,userPath string
	var uninitialized error

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
					Name:  "dev",
					Aliases: []string{"d"},
					Usage: "generate a default configuration files, suitable for editing",
					ArgsUsage: "holochain-name",
					Action: func(c *cli.Context) error {
						h,err := holo.GenDev(root+"/"+c.Args().First())
						if err == nil {
							if (verbose) {
								fmt.Printf("created %s with new id: %v\n",h.Id);
							}
						}
						return err
					},
				},
				{
					Name:  "keys",
					Aliases: []string{"k","key"},
					Usage: "generate separate key pair for entry signing on a specific holochain",
					ArgsUsage: "holochain-name",
					Action: func(c *cli.Context) error {
						if !initialized {return uninitialized}
						name := c.Args().First()
						chains := holo.ConfiguredChains(root)
						if !chains[name] {return errors.New(name+" doesn't exist")}
						return holo.GenKeys(root+"/"+name)
					},
				},
				{
					Name:  "chain",
					Aliases: []string{"c"},
					Usage: "generate the genesis blocks from the configuration and keys",
					ArgsUsage: "holochain-name",
					Action: func(c *cli.Context) error {
						if !initialized {return uninitialized}
						return holo.GenChain()
					},
				},
			},
		},
		{
			Name:    "init",
			Aliases: []string{"i"},
			Usage:   "boostrap the holochain service",
			ArgsUsage: "agent-id",
			Action:  func(c *cli.Context) error {
				agent := c.Args().First()
				if agent == "" {return errors.New("missing required agent-id argument to init")}
				err := holo.Init(userPath,holo.Agent(agent))
				if err == nil {
					fmt.Println("Holochain service initialized")
					if (verbose) {
						fmt.Println("    ~/.holochain directory created")
						fmt.Printf("    defaults stored to %s\n",holo.SysFileName)
						fmt.Println("    key-pair generated")
						fmt.Printf("    default agent \"%s\" stored to %s\n",holo.AgentFileName)
					}
				}
				return err
			},
		},
		{
			Name:    "link",
			Aliases: []string{"l"},
			Usage:   "add an entry onto the chain",
			Action:  func(c *cli.Context) error {
				if !initialized {return uninitialized}
				return errors.New("not implemented")
			},
		},
		{
			Name:    "status",
			Aliases: []string{"s"},
			Usage:   "display information about installed chains",
			Action:  func(c *cli.Context) error {
				if !initialized {return uninitialized}
				listChains(root)
				return nil
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		if (verbose) {
			fmt.Printf("app version: %s; Holochain lib version %s\n ",app.Version,holo.Version)
		}
		u,err := user.Current()
		if err != nil {return err}
		userPath = u.HomeDir
		root = userPath+"/"+holo.DirectoryName
		if initialized = holo.IsInitialized(userPath); !initialized {
			uninitialized = errors.New("service not initialized, run 'hc init'")
		}
		return nil
	}

	app.Action = func(c *cli.Context) error {
		if (!initialized) {
			cli.ShowAppHelp(c)
		} else {
			listChains(root)
		}
		return nil
	}

	app.Run(os.Args)
}

func listChains(root string) {
	chains := holo.ConfiguredChains(root)
	if len(chains) > 0 {
		fmt.Println("installed holochains: ")
		for k := range chains {
			fmt.Println("     ",k)
		}
	} else {
		fmt.Println("no installed chains")
	}
}
