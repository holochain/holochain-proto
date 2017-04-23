package main

import (
	"bytes"
	"errors"
	"fmt"
	holo "github.com/metacurrency/holochain"
	"github.com/urfave/cli"
	"os"
	"os/user"
	"sort"
	"strings"
	"time"
)

const (
	defaultPort = "3141"
)

var uninitialized error
var initialized bool

var debug bool
var verbose bool

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "hc"
	app.Usage = "holochain peer command line interface"
	app.Version = fmt.Sprintf("0.0.5 (holochain %s)", holo.VersionStr)

	var force bool
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
			Usage:     "bootstrap the holochain service",
			Action: func(c *cli.Context) error {
				agent := c.Args().First()
				if agent == "" {
					return errors.New("missing required agent-id argument to init")
				}
				_, err := holo.Init(root, holo.AgentName(agent))
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
			Name:      "clone",
			Aliases:   []string{"cl", "c"},
			ArgsUsage: "src-path holochain-name",
			Usage:     "clone a holochain instance from a source",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "force",
					Usage:       "overwrite existing holochain",
					Destination: &force,
				},
			},
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
				h, err := service.Clone(srcPath, root+"/"+name, true)
				if err == nil {
					if verbose {
						fmt.Printf("cloned %s from %s with new id: %v\n", name, srcPath, h.ID)
					}
				}
				return err
			},
		},
		{
			Name:    "test",
			Aliases: []string{"t"},
			Usage:   "run validation against test data for a chain in development",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "force",
					Usage:       "overwrite existing holochain",
					Destination: &force,
				},
			},
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "test")
				if err != nil {
					return err
				}
				if force {
					err = h.Reset()
					if err != nil {
						return err
					}
				}
				err = h.Activate()
				if err != nil {
					return err
				}

				args := c.Args()
				var errs []error

				if len(args) == 3 {
					dir := h.TestPath() + "/" + args[1]
					role := args[2]

					err, errs = h.TestScenario(dir, role)
					if err != nil {
						return err
					}
				} else if len(args) != 1 {
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
		{
			Name:    "genesis",
			Usage:   "generate genesis entries or keys for a cloned holochain",
			Aliases: []string{"gen", "g"},
			Subcommands: []cli.Command{
				{
					Name:      "chain",
					Aliases:   []string{"c"},
					ArgsUsage: "holochain-name",
					Usage:     "generate the genesis blocks from the configuration and keys",
					Action: func(c *cli.Context) error {
						name, err := checkForName(c, "gen chain")
						if err != nil {
							return err
						}

						err = genChain(service, name)
						return err
					},
				},
				{
					Name:      "keys",
					Aliases:   []string{"key", "k"},
					ArgsUsage: "holochain-name",
					Usage:     "generate separate key pair for entry signing on a specific holochain",
					Action: func(c *cli.Context) error {
						// need to implement this later when this would
						// check to see if the chain is started, and if so
						// actually add a new AgentEntry to a chain, otherwise
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
							err = holo.SaveAgent(h.rootPath, h.agent)
							return err*/

					},
				},
			},
		},
		{
			Name:      "web",
			Aliases:   []string{"serve", "w"},
			ArgsUsage: "holochain-name [port]",
			Usage:     fmt.Sprintf("serve a chain to the web on localhost:<port> (defaults to %s)", defaultPort),
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "serve")
				if err != nil {
					return err
				}
				if !h.Started() {
					return fmt.Errorf("Can't serve an un-started chain. Run 'gen chain %s' to generate genesis entries and start the chain.", h.Name)
				}

				if verbose {
					fmt.Printf("Serving holochain with DNA hash:%v\n", h.DNAHash())
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
				go h.DHT().HandleChangeReqs()
				go h.DHT().Gossip(2 * time.Second)
				serve(h, port)
				return err
			},
		},
		{
			Name:      "call",
			Aliases:   []string{"ca"},
			ArgsUsage: "holochain-name zome-name function args",
			Usage:     "call an exposed function",
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
			Name:      "dump",
			Aliases:   []string{"d"},
			ArgsUsage: "holochain-name",
			Usage:     "display a text dump of a chain",
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "dump")
				if err != nil {
					return err
				}

				if !h.Started() {
					return errors.New("No data to dump, chain not yet initialized.")
				}
				dnaHash := h.DNAHash()

				fmt.Printf("Chain: %s\n", dnaHash)

				links := make(map[string]holo.Header)
				index := make(map[int]string)
				entries := make(map[int]interface{})
				idx := 0
				err = h.Walk(func(key *holo.Hash, header *holo.Header, entry holo.Entry) (err error) {
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
					case holo.KeyEntryType:
						fmt.Printf("       %v\n", e.(*holo.GobEntry).C)
					case holo.DNAEntryType:
						fmt.Printf("       %s\n", e.(*holo.GobEntry).C)
					case holo.AgentEntryType:
						fmt.Printf("       %v\n", e.(*holo.GobEntry).C.(holo.AgentEntry))
					default:
						fmt.Printf("       %v\n", e)
					}
				}
				return nil
			},
		},
		{
			Name:      "join",
			Aliases:   []string{"c"},
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
				_, err := service.Clone(srcPath, root+"/"+name, false)
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
			Name:      "reset",
			Aliases:   []string{"r"},
			ArgsUsage: "holochain-name",
			Usage:     "reset a chain. Warning this destroys all chain data!",
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "reset")
				if err != nil {
					return err
				}
				err = h.Reset()
				return err
			},
		},
		{
			Name:      "seed",
			ArgsUsage: "holochain-name",
			Usage:     "seed calculates DNA hashes and builds DNA file without generating genesis entries.  Useful only for testing and development.",
			Action: func(c *cli.Context) error {
				h, err := getHolochain(c, service, "seed")
				if err != nil {
					return err
				}
				err = h.GenDNAHashes()
				if err != nil {
					return err
				}
				var buf bytes.Buffer
				err = h.EncodeDNA(&buf)
				if err != nil {
					return err
				}
				e := holo.GobEntry{C: buf.Bytes()}
				hash, err := e.Sum(h.HashSpec())
				fmt.Printf("holochain id:%v\n", hash)
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
			Name:      "template",
			Aliases:   []string{"dev", "t"},
			ArgsUsage: "holochain-name [template]",
			Usage:     "generate a configuration file template suitable for editing",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "force",
					Usage:       "overwrite existing holochain",
					Destination: &force,
				},
			},
			Action: func(c *cli.Context) error {
				name, err := checkForName(c, "template")
				if err != nil {
					return err
				}
				format := "toml"
				if len(c.Args()) == 2 {
					format = c.Args()[1]
					if !(format == "json" || format == "yaml" || format == "toml") {
						return errors.New("template: format must be one of yaml,toml,json")
					}
				}
				if force {
					e := os.RemoveAll(root + "/" + name)
					if e != nil {
						return e
					}
				}
				h, err := service.GenDev(root+"/"+name, format)
				if err == nil {
					if verbose {
						fmt.Printf("created %s with new id: %v\n", name, h.ID)
					}
				}
				return err
			},
		},
	}

	app.Before = func(c *cli.Context) error {
		if debug {
			os.Setenv("DEBUG", "1")
		}
		holo.Initialize()
		if verbose {
			fmt.Printf("hc version %s \n", app.Version)
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

	err := app.Run(os.Args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
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
		err = errors.New("missing required holochain-name argument to " + cmd)
	}
	return
}

func listChains(s *holo.Service) {
	chains, _ := s.ConfiguredChains()
	l := len(chains)
	if l > 0 {
		keys := make([]string, l)
		i := 0
		for k := range chains {
			keys[i] = k
			i++
		}
		sort.Strings(keys)
		fmt.Println("installed holochains: ")
		for _, k := range keys {
			id := chains[k].DNAHash()
			var sid = "<not-started>"
			if id.String() != "" {
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

func genChain(service *holo.Service, name string) error {
	h, err := service.Load(name)
	if err != nil {
		return err
	}
	err = h.GenDNAHashes()
	if err != nil {
		return err
	}
	err = h.Activate()
	if err != nil {
		return err
	}
	_, err = h.GenChain()
	if err != nil {
		return err
	}
	go h.DHT().HandleChangeReqs()

	if verbose {
		fmt.Printf("Genesis entries added and DNA hashed for new holochain with ID: %s\n", h.DNAHash().String())
	}
	return nil
}
