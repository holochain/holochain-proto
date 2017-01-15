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
	var service *holo.Service

	holo.Register()

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
						name := c.Args().First()
						if name == "" {return errors.New("missing require holochain-name argument to gen dev")}
						h,err := holo.GenDev(root+"/"+name)
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
						chains,_ := service.ConfiguredChains()
						if chains[name]==nil {return errors.New(name+" doesn't exist")}
						_,err := holo.GenKeys(root+"/"+name)
						return err
					},
				},
				{
					Name:  "chain",
					Aliases: []string{"c"},
					Usage: "generate the genesis blocks from the configuration and keys",
					ArgsUsage: "holochain-name",
					Action: func(c *cli.Context) error {
						if !initialized {return uninitialized}
						name := c.Args().First()
						if name == "" {return errors.New("missing require holochain-name argument to gen chain")}
						h,err := service.Load(name)
						if err != nil {return err}
						err = h.GenDNAHashes()
						if err != nil {return err}
						_,err = h.GenChain()
						if err != nil {return err}
						id,err := h.ID()
						if err != nil {return err}

						fmt.Printf("Genesis entries added and DNA hashed for new holochain with ID: %s\n",id.String())
						return nil
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
				_,err := holo.Init(userPath,holo.Agent(agent))
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
			Name:    "dump",
			Aliases: []string{"d"},
			Usage:   "display a text dump of a chain",
			ArgsUsage: "holochain-name",
			Action:  func(c *cli.Context) error {
				if !initialized {return uninitialized}
				name := c.Args().First()
				if name == "" {return errors.New("missing require holochain-name argument to dump")}
				h,err := service.IsConfigured(name)
				if err != nil {return err}

				id,err := h.ID()
				if err != nil {return err}
				fmt.Printf("Chain: %s\n",id)

				links := make(map[string]holo.Header,0)
				index := make(map[int]string,0)
				idx := 0
				err = h.Walk(func(key *holo.Hash,header *holo.Header,entry interface{})(err error){
					ks := (*key).String()
					index[idx] = ks
					links[ks] = *header
					idx++
					return nil
				},false)

				for i:=0;i<idx;i++ {
					k := index[i]
					hdr := links[k]
					fmt.Printf("%s:%s @ %v\n",hdr.Type,k,hdr.Time)
					fmt.Printf("    Next Header: %v\n",hdr.HeaderLink)
					fmt.Printf("          Entry: %v\n",hdr.EntryLink)
				}
				return nil
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
				listChains(service)
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
		} else {
			service,err = holo.LoadService(root)
		}
		return err
	}

	app.Action = func(c *cli.Context) error {
		if (!initialized) {
			cli.ShowAppHelp(c)
		} else {
			listChains(service)
		}
		return nil
	}

	app.Run(os.Args)
}

func listChains(s *holo.Service) {
	chains,_ := s.ConfiguredChains()
	if len(chains) > 0 {
		fmt.Println("installed holochains: ")
		for k := range chains {
			id,err := chains[k].ID()
			var sid = "<not-started>"
			if err == nil {
				sid = id.String()
			}
			fmt.Println("    ",k,sid)
		}
	} else {
		fmt.Println("no installed chains")
	}
}
