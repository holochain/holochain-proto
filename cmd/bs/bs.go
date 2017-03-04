package main

import (
	"encoding/json"
	"errors"
	"fmt"
	holo "github.com/metacurrency/holochain"
	"github.com/op/go-logging"
	"github.com/tidwall/buntdb"
	"github.com/urfave/cli"
	"net/http"
	"os"
	"os/user"
	"strings"
)

const (
	DefaultPort = 3142
)

var log = logging.MustGetLogger("main")

var store *buntdb.DB

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "bs"
	app.Usage = "holochain bootstrap server"
	app.Version = "0.0.1"

	var port int
	var dbpath string

	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:        "port",
			Usage:       "bootstrap server port",
			Value:       DefaultPort,
			Destination: &port,
		},
	}

	app.Before = func(c *cli.Context) error {

		log.Infof("app version: %s; Holochain bootstrap server", app.Version)

		var err error
		if dbpath == "" {
			dbpath = os.Getenv("HOLOBSPATH")
			if dbpath == "" {
				u, err := user.Current()
				if err != nil {
					return err
				}
				userPath := u.HomeDir
				dbpath = userPath + "/.hcboostrapdb"
			}
		}
		store, err = buntdb.Open(dbpath)
		if err != nil {
			panic(err)
		}
		store.CreateIndex("chain", "*", buntdb.IndexJSON("HID"))

		return err
	}

	app.Action = func(c *cli.Context) error {
		return serve(port)
	}
	return
}

func main() {
	app := setupApp()
	app.Run(os.Args)
}

type Node struct {
	Req    holo.BSReq
	Remote string
	HID    string
}

func h(w http.ResponseWriter, r *http.Request) {
	var err error
	log.Infof("%s: processing req:%s\n", r.Method, r.URL.Path)
	path := strings.Split(r.URL.Path, "/")
	if r.Method == "GET" {
		if len(path) != 2 {
			http.Error(w, "expecting path /<holochainid>", 400)
			return
		}
		chain := string(path[1])

		err = store.View(func(tx *buntdb.Tx) error {
			nodes := make([]holo.BSResp, 0)
			//hid := fmt.Sprintf(`{"HID":"%s"}`, chain)

			tx.Ascend("chain", func(key, value string) bool {
				var nd Node
				json.Unmarshal([]byte(value), &nd)
				if nd.HID == chain {
					log.Infof("Found: %s=>%s", key, value)
					resp := holo.BSResp{Req: nd.Req, Remote: nd.Remote}
					nodes = append(nodes, resp)
				}
				return true
			})
			var b []byte
			b, err = json.Marshal(nodes)
			if err == nil {
				fmt.Fprintf(w, string(b))
			}

			return err
		})
	} else if r.Method == "POST" {
		if len(path) != 3 {
			http.Error(w, "expecting path /<holochainid>/<peerid>", 400)
			return
		}
		chain := string(path[1])
		node := string(path[2])

		var req holo.BSReq
		if r.Body == nil {
			err = errors.New("Please send a request body")
		}
		if err == nil {
			err = json.NewDecoder(r.Body).Decode(&req)
			if err == nil {
				err = store.Update(func(tx *buntdb.Tx) error {
					var b []byte
					n := Node{Remote: r.RemoteAddr, Req: req, HID: chain}
					b, err = json.Marshal(n)
					if err == nil {
						_, _, err = tx.Set(node, string(b), nil)
						if err == nil {
							log.Infof("Set: %s", string(b))
							fmt.Fprintf(w, "ok")
						}
					}
					return err
				})
			}
		}
	}
	if err != nil {
		log.Infof("Error:%s", err.Error())
		http.Error(w, err.Error(), 400)
	} else {
		log.Infof("Success")
	}
}

func serve(port int) (err error) {
	http.HandleFunc("/", h)

	log.Infof("starting up on port %d", port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil) // set listen port
	return
}
