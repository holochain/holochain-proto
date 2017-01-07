package main

import (
	"fmt"
	"os"
	"os/user"
	"errors"
	"io/ioutil"

	holo "github.com/metacurrency/holochain"
	"github.com/urfave/cli"

	//"github.com/google/uuid"
)


func main() {
	app := cli.NewApp()
	app.Name = "hc"
	app.Usage = "holochain command line interface"
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
					Usage: "generate a default configuration files, suit",
					ArgsUsage: "name",
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
			Action:  func(c *cli.Context) error {
				err := holo.Init(userPath)
				if err == nil {
					fmt.Println("Holochain service initialized")
					if (verbose) {
						fmt.Println("    ~/.holochain directory created")
						fmt.Println("    default system.conf generated")
						fmt.Println("    key-pair generated")
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
	files, _ := ioutil.ReadDir(root)
	chains := make([]string,0)
	for _, f := range files {
		if f.IsDir() && holo.IsConfigured(root+"/"+f.Name()) {
			chains = append(chains,f.Name())
		}
	}
	if len(chains) > 0 {
		fmt.Println("installed holochains: ",chains)
	} else {
		fmt.Println("no installed chains")
	}
}
