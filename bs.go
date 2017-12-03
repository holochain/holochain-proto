// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements bootstrap server access

package holochain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const (
	BootstrapTTL = time.Minute * 5
)

type BSReq struct {
	Version  int
	NodeID   string
	NodeAddr string
}

type BSResp struct {
	Req      BSReq
	Remote   string
	LastSeen time.Time
}

func (h *Holochain) BSpost() (err error) {
	if h.node == nil {
		return errors.New("Node hasn't been initialized yet.")
	}
	nodeID := h.nodeIDStr
	req := BSReq{Version: 1, NodeID: nodeID, NodeAddr: h.node.ExternalAddr().String()}
	host := h.Config.BootstrapServer
	id := h.DNAHash()
	url := fmt.Sprintf("http://%s/%s/%s", host, id.String(), nodeID)
	var b []byte
	b, err = json.Marshal(req)
	//var resp *http.Response
	if err == nil {
		var resp *http.Response
		resp, err = http.Post(url, "application/json", bytes.NewBuffer(b))
		if err == nil {
			resp.Body.Close()
		}

	}
	return
}

func (h *Holochain) checkBSResponses(nodes []BSResp) (err error) {
	myNodeID := h.nodeIDStr
	for _, r := range nodes {
		h.dht.dlog.Logf("checking returned node: %v", r)

		var id peer.ID
		var addr ma.Multiaddr
		id, err = peer.IDB58Decode(r.Req.NodeID)
		if err == nil {
			//@TODO figure when to use Remote or r.NodeAddr
			x := strings.Split(r.Remote, ":")
			y := strings.Split(r.Req.NodeAddr, "/")
			port := y[len(y)-1]

			// assume the multi-address is the ip address as the bootstrap server saw it
			// with port number advertised by the node in it's multi-address

			addr, err = ma.NewMultiaddr("/ip4/" + x[0] + "/tcp/" + port)
			if err == nil {
				// don't "discover" ourselves
				if r.Req.NodeID != myNodeID {
					h.dht.dlog.Logf("discovered peer via bs: %s (%v)", r.Req.NodeID, addr)
					go func() {
						err = h.AddPeer(pstore.PeerInfo{ID: id, Addrs: []ma.Multiaddr{addr}})
					}()
				}

			}
		}
	}
	return
}

func (h *Holochain) BSget() (err error) {
	if h.node == nil {
		return errors.New("Node hasn't been initialized yet.")
	}
	host := h.Config.BootstrapServer
	if host == "" {
		return
	}
	id := h.DNAHash()
	url := fmt.Sprintf("http://%s/%s", host, id.String())

	var req *http.Request

	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Close = true
	client := http.DefaultClient
	var resp *http.Response
	resp, err = client.Do(req)

	//	resp, err = http.Get(url)
	if err == nil {
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		if err == nil {
			var nodes []BSResp
			err = json.Unmarshal(b, &nodes)
			if err == nil {
				err = h.checkBSResponses(nodes)

			}
		}
		resp.Body.Close()

	}
	return
}
