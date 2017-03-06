package main

import (
	"errors"
	"fmt"
	holo "github.com/metacurrency/holochain"
	"github.com/op/go-logging"
	"github.com/urfave/cli"
	"os"
	"os/user"
	"strings"
)

var uninitialized error
var initialized bool
var log *logging.Logger

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "hc"
	app.Usage = "holochain peer command line interface"
	app.Version = "0.0.1"
	var verbose bool
	var debug bool
	var force bool
	var root string
	var service *holo.Service

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "verbose",
			Usage:       "verbose output",
			Destination: &verbose,
		},
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
	}

	app.Commands = []cli.Command{
		{
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "force",
					Usage:       "overwrite existing holochain",
					Destination: &force,
				},
			},
			Name:      "clone",
			Aliases:   []string{"c"},
			Usage:     "clone a holochain instance from a source",
			ArgsUsage: "src-path holochain-name",
			Action: func(c *cli.Context) error {
				srcPath := c.Args().First()
				if srcPath == "" {
					return errors.New("clone: missing required source path argument")
				}
				if len(c.Args()) == 1 {
					return errors.New("clone: missing required holochain-name argument")
				}
				name := c.Args()[1]
				if force {
					e := os.RemoveAll(root + "/" + name)
					if e != nil {
						return e
					}
				}
				h, err := service.Clone(srcPath, root+"/"+name)
				if err == nil {
					if verbose {
						fmt.Printf("cloned %s from %s with new id: %v\n", name, srcPath, h.Id)
					}
				}
				return err
			},
		},
		{
			Name:    "gen",
			Usage:   "generate genesis entries or keys for a cloned holochain",
			Aliases: []string{"g"},
			Subcommands: []cli.Command{
				{
					Name:      "dev",
					Aliases:   []string{"d"},
					Usage:     "generate a default configuration files, suitable for editing",
					ArgsUsage: "holochain-name [dna-format]",
					Action: func(c *cli.Context) error {
						name, err := checkForName(c, "gen dev")
						if err != nil {
							return err
						}
						format := "toml"
						if len(c.Args()) == 2 {
							format = c.Args()[1]
							if !(format == "json" || format == "yaml" || format == "toml") {
								return errors.New("gen dev: format must be one of yaml,toml,json")
							}
						}
						h, err := service.GenDev(root+"/"+name, format)
						if err == nil {
							if verbose {
								fmt.Printf("created %s with new id: %v\n", name, h.Id)
							}
						}
						return err
					},
				},
				{
					Name:      "keys",
					Aliases:   []string{"k", "key"},
					Usage:     "generate separate key pair for entry signing on a specific holochain",
					ArgsUsage: "holochain-name",
					Action: func(c *cli.Context) error {
						// need to implement this later when this would
						// check to see if the chain is started, and if so
						// actually add a new KeyEntry to a chain, otherwise
						// it could just add chain specific files.
						return errors.New("not yet implemented")
						/*
							name, err := checkForName(c, "gen keys")
							if err != nil {
								return err
							}
							h, err := service.Load(name)
							if err != nil {
								return err
							}
							h.agent.GenKeys()
							err = holo.SaveAgent(h.path, h.agent)
							return err*/

					},
				},
				{
					Name:      "chain",
					Aliases:   []string{"c"},
					Usage:     "generate the genesis blocks from the configuration and keys",
					ArgsUsage: "holochain-name",
					Action: func(c *cli.Context) error {
						name, err := checkForName(c, "gen chain")
						if err != nil {
							return err
						}
						h, err := service.Load(name)
						if err != nil {
							return err
						}
						err = h.GenDNAHashes()
						if err != nil {
							return err
						}
						_, err = h.GenChain()
						if err != nil {
							return err
						}
						id, err := h.ID()
						if err != nil {
							return err
						}

						fmt.Printf("Genesis entries added and DNA hashed for new holochain with ID: %s\n", id.String())
						return nil
					},
				},
			},
		},
		{
			Name:      "init",
			Aliases:   []string{"i"},
			Usage:     "boostrap the holochain service",
			ArgsUsage: "agent-id",
			Action: func(c *cli.Context) error {
				agent := c.Args().First()
				if agent == "" {
					return errors.New("missing required agent-id argument to init")
				}
				_, err := holo.Init(root, holo.AgentID(agent))
				if err == nil {
					fmt.Println("Holochain service initialized")
					if verbose {
						fmt.Println("    ~/.holochain directory created")
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
			Usage:     "display a text dump of a chain",
			ArgsUsage: "holochain-name",
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "dump")
				if err != nil {
					return err
				}

				id, err := h.ID()
				if err.Error() == "holochain: Meta key 'id' uninitialized" {
					return errors.New("No data to dump, chain not yet initialized.")
				}
				if err != nil {
					return err
				}
				fmt.Printf("Chain: %s\n", id)

				links := make(map[string]holo.Header)
				index := make(map[int]string)
				entries := make(map[int]interface{})
				idx := 0
				err = h.Walk(func(key *holo.Hash, header *holo.Header, entry interface{}) (err error) {
					ks := (*key).String()
					index[idx] = ks
					links[ks] = *header
					entries[idx] = entry
					idx++
					return nil
				}, true)

				for i := 0; i < idx; i++ {
					k := index[i]
					hdr := links[k]
					fmt.Printf("%s:%s @ %v\n", hdr.Type, k, hdr.Time)
					fmt.Printf("    Next Header: %v\n", hdr.HeaderLink)
					fmt.Printf("    Next %s: %v\n", hdr.Type, hdr.TypeLink)
					fmt.Printf("    Entry: %v\n", hdr.EntryLink)
					e := entries[i]
					switch hdr.Type {
					case holo.DNAEntryType:
						fmt.Printf("       %s\n", string(e.([]byte)))
					case holo.KeyEntryType:
						fmt.Printf("       %v\n", e.(holo.KeyEntry))
					default:
						fmt.Printf("       %v\n", e)
					}
				}
				return nil
			},
		},
		{
			Name:    "test",
			Aliases: []string{"t"},
			Usage:   "run validation against test data for a chain in development",
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "test")
				if err != nil {
					return err
				}
				err = h.Test()
				return err
			},
		},
		{
			Name:    "status",
			Aliases: []string{"s"},
			Usage:   "display information about installed chains",
			Action: func(c *cli.Context) error {
				if !initialized {
					return uninitialized
				}
				listChains(service)
				return nil
			},
		},
		{
			Name:      "call",
			Aliases:   []string{"c"},
			Usage:     "call an exposed function",
			ArgsUsage: "holochain-name zome-name function args",
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "call")
				if err != nil {
					return err
				}
				zome := os.Args[3]
				function := os.Args[4]
				args := os.Args[5:]
				fmt.Printf("calling %s on zome %s with params %v\n", function, zome, args)
				result, err := h.Call(zome, function, strings.Join(args, " "))
				if err != nil {
					return err
				}
				fmt.Printf("%v\n", result)
				return nil
			},
		},
		{
			Name:      "bs",
			Aliases:   []string{"b"},
			Usage:     "send bootstrap tickler to the chain bootstrap server",
			ArgsUsage: "bs",
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "bs")
				if err != nil {
					return err
				}
				err = h.BSpost()
				return err
			},
		},
		{
			Name:      "serve",
			Aliases:   []string{"w"},
			Usage:     "serve a chain to the web",
			ArgsUsage: "holochain-name [port]",
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "serve")
				if err != nil {
					return err
				}
				var port string
				if len(c.Args()) == 1 {
					port = "3141"
				} else {
					port = c.Args()[1]
				}
				serve(h, port)
				return err
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		level := logging.INFO
		if debug {
			level = logging.DEBUG
		}
		log = logging.MustGetLogger("holochain")
		logging.SetLevel(level, "holochain")
		holo.Register(log)
		if verbose {
			fmt.Printf("app version: %s; Holochain lib version %s\n ", app.Version, holo.Version)
		}
		var err error
		if root == "" {
			root = os.Getenv("HOLOPATH")
			if root == "" {
				u, err := user.Current()
				if err != nil {
					return err
				}
				userPath := u.HomeDir
				root = userPath + "/" + holo.DefaultDirectoryName
			}
		}
		if initialized = holo.IsInitialized(root); !initialized {
			uninitialized = errors.New("service not initialized, run 'hc init'")
		} else {
			service, err = holo.LoadService(root)
		}
		return err
	}

	app.Action = func(c *cli.Context) error {
		if !initialized {
			cli.ShowAppHelp(c)
		} else {
			listChains(service)
		}
		return nil
	}
	return
}

func main() {
	app := setupApp()
	app.Run(os.Args)
}

func getHolochain(c *cli.Context, service *holo.Service, cmd string) (h *holo.Holochain, err error) {
	name, err := checkForName(c, cmd)
	if err != nil {
		return
	}
	h, err = service.Load(name)
	if err != nil {
		return
	}
	return
}

func checkForName(c *cli.Context, cmd string) (name string, err error) {
	if !initialized {
		err = uninitialized
		return
	}
	name = c.Args().First()
	if name == "" {
		err = errors.New("missing require holochain-name argument to " + cmd)
	}
	return
}

func listChains(s *holo.Service) {
	chains, _ := s.ConfiguredChains()
	if len(chains) > 0 {
		fmt.Println("installed holochains: ")
		for k := range chains {
			id, err := chains[k].ID()
			var sid = "<not-started>"
			if err == nil {
				sid = id.String()
			}
			fmt.Println("    ", k, sid)
		}
	} else {
		fmt.Println("no installed chains")
	}
}

func mkErr(etext string, code int) (int, error) {
	fmt.Println("Error:", code, etext)
	return code, errors.New(etext)
}
