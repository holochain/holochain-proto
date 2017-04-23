// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements webserver functionality for the hc command

package main

import (
	_ "encoding/json"
	"fmt"
	websocket "github.com/gorilla/websocket"
	holo "github.com/metacurrency/holochain"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

var log = holo.Logger{Format: "%{color:magenta}%{message}"}
var errs = holo.Logger{Format: "%{color:red}%{time} %{message}", Enabled: true}

func serve(h *holo.Holochain, port string) {

	log.New(nil)
	errs.New(os.Stderr)

	fs := http.FileServer(http.Dir(h.UIPath()))
	http.Handle("/", fs)

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	http.HandleFunc("/_sock/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			errs.Log(err)
			return
		}

		for {
			var v map[string]string
			err := conn.ReadJSON(&v)

			log.Logf("conn got: %v\n", v)

			if err != nil {
				errs.Log(err)
				return
			}
			zome := v["zome"]
			function := v["fn"]
			result, err := call(h, zome, function, v["arg"])
			switch t := result.(type) {
			case string:
				err = conn.WriteMessage(websocket.TextMessage, []byte(t))
			case []byte:
				err = conn.WriteMessage(websocket.TextMessage, t)
				//err = conn.WriteJSON(t)
			default:
				err = fmt.Errorf("Unknown type from Call of %s:%s", zome, function)
			}

			if err != nil {
				errs.Log(err)
				return
			}
		}
	})

	http.HandleFunc("/fn/", func(w http.ResponseWriter, r *http.Request) {

		var err error
		var errCode = 400
		defer func() {
			if err != nil {
				log.Logf("ERROR:%s,code:%d", err.Error(), errCode)
				http.Error(w, err.Error(), errCode)
			}
		}()

		/*		if r.Method == "GET" {
					fmt.Printf("processing Get:%s\n", r.URL.Path)

					http.Redirect(w, r, "/static", http.StatusSeeOther)
				}
		*/
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errCode, err = mkErr("unable to read body", 500)
			return
		}
		fmt.Printf("processing req:%s\n  Body:%v\n", r.URL.Path, string(body))

		path := strings.Split(r.URL.Path, "/")

		zome := path[2]
		function := path[3]
		args := string(body)
		result, err := call(h, zome, function, args)
		if err != nil {
			log.Logf("HC Serve: call of %s:%s resulted in error: %v\n", zome, function, err)
			http.Error(w, err.Error(), 500)

			return
		}
		log.Logf(" result: %v\n", result)
		switch t := result.(type) {
		case string:
			fmt.Fprintf(w, t)
		case []byte:
			fmt.Fprintf(w, string(t))
		default:
			err = fmt.Errorf("Unknown type from Call of %s:%s", zome, function)
		}
	}) // set router
	fmt.Printf("starting server on localhost:%s\n", port)
	err := http.ListenAndServe(":"+port, nil) // set listen port
	if err != nil {
		errs.Logf("Couldn't start server: %v", err)
	}
}

func call(h *holo.Holochain, zome string, function string, args string) (result interface{}, err error) {

	log.Logf("calling %s:%s(%s)\n", zome, function, args)
	result, err = h.Call(zome, function, args)

	if err != nil {
		_, err = mkErr(err.Error(), 400)
	}
	return
}
