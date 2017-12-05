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
	"time"
)

const (
	DefaultPort = 3142
)

var log = logging.MustGetLogger("main")

var store *buntdb.DB

func setupDB(dbpath string) (err error) {
	if dbpath == "" {
		dbpath = os.Getenv("HOLOBSPATH")
		if dbpath == "" {
			u, err := user.Current()
			if err != nil {
				return err
			}
			userPath := u.HomeDir
			dbpath = userPath + "/.hcbootstrapdb"
		}
	}
	store, err = buntdb.Open(dbpath)
	if err != nil {
		panic(err)
	}
	store.CreateIndex("chain", "*", buntdb.IndexJSON("HID"))
	return
}

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "bs"
	app.Usage = "holochain bootstrap server"
	app.Version = "0.0.2"

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

		err := setupDB(dbpath)
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
	Req      holo.BSReq
	Remote   string
	HID      string
	LastSeen time.Time
}

func get(chain string) (result string, err error) {
	err = store.View(func(tx *buntdb.Tx) error {
		nodes := make([]holo.BSResp, 0)
		//hid := fmt.Sprintf(`{"HID":"%s"}`, chain)

		now := time.Now()
		tx.Ascend("chain", func(key, value string) bool {
			var nd Node
			json.Unmarshal([]byte(value), &nd)
			if nd.HID == chain {
				if nd.LastSeen.Add(holo.BootstrapTTL).After(now) {
					log.Infof("Found: %s=>%s", key, value)
					resp := holo.BSResp{Req: nd.Req, Remote: nd.Remote, LastSeen: nd.LastSeen}
					nodes = append(nodes, resp)
				}
			}
			return true
		})
		var b []byte
		b, err = json.Marshal(nodes)
		if err == nil {
			result = string(b)
		}
		return err
	})
	return
}

func post(chain string, req *holo.BSReq, remote string, seen time.Time) (err error) {
	err = store.Update(func(tx *buntdb.Tx) error {
		var b []byte
		n := Node{Remote: remote, Req: *req, HID: chain, LastSeen: seen}
		b, err = json.Marshal(n)
		if err == nil {
			key := chain + ":" + req.NodeID
			_, _, err = tx.Set(key, string(b), nil)
			if err == nil {
				log.Infof("Set: %s", string(b))
			}
		}
		return err
	})
	return
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
		var result string
		result, err = get(chain)
		if err == nil {
			fmt.Fprintf(w, result)
		}
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
				if req.NodeID != node {
					err = errors.New("id in post URL doesn't match Req")
				} else {
					err = post(chain, &req, r.RemoteAddr, time.Now())
					if err == nil {
						fmt.Fprintf(w, "ok")
					}
				}
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

func getCompleteConnectionList(response http.ResponseWriter, request *http.Request) {
	var err error
	err = store.View(func(tx *buntdb.Tx) error {
		nodes := make([]holo.BSResp, 0)
		//hid := fmt.Sprintf(`{"HID":"%s"}`, chain)

		tx.Ascend("chain", func(key, value string) bool {
			var node Node
			json.Unmarshal([]byte(value), &node)

			log.Infof("Found: %s=>%s", key, value)
			resp := holo.BSResp{Req: node.Req, Remote: node.Remote}

			nodes = append(nodes, resp)

			return true
		})
		var b []byte
		b, err = json.Marshal(nodes)
		if err == nil {
			fmt.Fprintf(response, string(b))
			fmt.Fprintf(response, string("oK"))
		}

		return err
	})
}

func serve(port int) (err error) {

	mux := http.NewServeMux()

	mux.HandleFunc("/", h)
	mux.HandleFunc("/getCompleteConnectionList", getCompleteConnectionList)

	log.Infof("starting up on port %d", port)

	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", port), // set listen port
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	err = s.ListenAndServe()
	return
}
