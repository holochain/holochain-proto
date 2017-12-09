// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements webserver functionality for holochain UI

package ui

import (
	"context"
	_ "encoding/json"
	"errors"
	"fmt"
	websocket "github.com/gorilla/websocket"
	holo "github.com/metacurrency/holochain"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type WebServer struct {
	h      *holo.Holochain
	port   string
	log    holo.Logger
	errs   holo.Logger
	stop   chan bool
	server *http.Server
}

func NewWebServer(h *holo.Holochain, port string) *WebServer {
	w := WebServer{h: h, port: port}
	w.log = holo.Logger{Format: "%{color:magenta}%{message}"}
	w.errs = holo.Logger{Format: "%{color:red}%{time} %{message}", Enabled: true}
	w.stop = make(chan bool, 1)
	return &w
}

//Start starts up a web server and returns a channel which will shutdown
func (ws *WebServer) Start() {

	mux := http.NewServeMux()

	ws.log.New(nil)
	ws.errs.New(os.Stderr)

	fs := http.FileServer(http.Dir(ws.h.UIPath()))
	mux.Handle("/", fs)

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	mux.HandleFunc("/_sock/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			ws.errs.Logf(err.Error())
			return
		}

		for {
			var v map[string]string
			err := conn.ReadJSON(&v)

			ws.log.Logf("conn got: %v\n", v)

			if err != nil {
				ws.errs.Log(err)
				return
			}
			zome := v["zome"]
			function := v["fn"]
			result, err := ws.call(zome, function, v["arg"])
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
				ws.errs.Log(err)
				return
			}
		}
	})

	mux.HandleFunc("/fn/", func(w http.ResponseWriter, r *http.Request) {

		var err error
		var errCode = 400
		defer func() {
			if err != nil {
				ws.log.Logf("ERROR:%s,code:%d", err.Error(), errCode)
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
		ws.log.Logf("processing req:%s\n  Body:%v\n", r.URL.Path, string(body))

		path := strings.Split(r.URL.Path, "/")
		if len(path) != 4 {
			errCode, err = mkErr("bad request", 400)
			return
		}
		zome := path[2]
		function := path[3]
		args := string(body)
		result, err := ws.call(zome, function, args)
		if err != nil {
			ws.log.Logf("call of %s:%s resulted in error: %v\n", zome, function, err)
			http.Error(w, err.Error(), 500)

			return
		}
		ws.log.Logf(" result: %v\n", result)
		switch t := result.(type) {
		case string:
			fmt.Fprint(w, t)
		case []byte:
			fmt.Fprint(w, string(t))
		default:
			err = fmt.Errorf("Unknown type from Call of %s:%s", zome, function)
		}
	})

	mux.HandleFunc("/bridge/", func(w http.ResponseWriter, r *http.Request) {

		var err error
		var errCode = 400
		defer func() {
			if err != nil {
				ws.log.Logf("ERROR:%s,code:%d", err.Error(), errCode)
				http.Error(w, err.Error(), errCode)
			}
		}()

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			errCode, err = mkErr("unable to read body", 500)
			return
		}
		ws.log.Logf("processing req:%s\n  Body:%v\n", r.URL.Path, string(body))

		path := strings.Split(r.URL.Path, "/")
		if len(path) != 5 {
			errCode, err = mkErr("bad request", 400)
			return
		}
		token := path[2]
		zome := path[3]
		function := path[4]
		args := string(body)

		ws.log.Logf("bridge calling %s:%s(%s)\n", zome, function, args)
		result, err := ws.h.BridgeCall(zome, function, args, token)
		if err != nil {
			ws.log.Logf("call of %s:%s resulted in error: %v\n", zome, function, err)
			errCode, err = mkErr(err.Error(), 400)
			return
		}
		ws.log.Logf(" result: %v\n", result)
		switch t := result.(type) {
		case string:
			fmt.Fprint(w, t)
		case []byte:
			fmt.Fprint(w, string(t))
		default:
			err = fmt.Errorf("Unknown type from Call of %s:%s", zome, function)
		}
	})

	// set router
	ws.log.Logf("Starting server on localhost:%s\n", ws.port)

	ws.server = &http.Server{Addr: ":" + ws.port, Handler: mux}

	go func() {
		if err := ws.server.ListenAndServe(); err != nil {
			// when the server is stopped by Shutdown() ListenAndServe returns with ErrServerClosed
			if err != http.ErrServerClosed {
				ws.errs.Logf("Couldn't start server: %v", err)
			} else {
				ws.log.Logf("Server closed")
			}
			ws.stop <- true // set the channel to make sure it unblocks
		}
	}()
}

// Stop sends a message through the stop channel to unblock
func (ws *WebServer) Stop() {
	ws.stop <- true
}

// Wait blocks on the stop channel and when it finishes shuts down the server
func (ws *WebServer) Wait() {
	<-ws.stop
	if ws.server != nil {
		ws.server.Shutdown(context.Background())
		ws.server = nil
	}
}

func mkErr(etext string, code int) (int, error) {
	return code, errors.New(etext)
}

func (ws *WebServer) call(zome string, function string, args string) (result interface{}, err error) {

	ws.log.Logf("calling %s:%s(%s)\n", zome, function, args)
	result, err = ws.h.Call(zome, function, args, holo.PUBLIC_EXPOSURE)

	if err != nil {
		_, err = mkErr(err.Error(), 400)
	}
	return
}
