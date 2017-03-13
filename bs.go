// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements bootstrap server access

package holochain

import (
	"bytes"
	"encoding/json"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	"io/ioutil"
	"net/http"
)

type BSReq struct {
	Version  int
	NodeID   string
	NodeAddr string
}

type BSResp struct {
	Req    BSReq
	Remote string
}

func (h *Holochain) BSpost() (err error) {
	nodeID := peer.IDB58Encode(h.node.HashAddr)
	req := BSReq{Version: 1, NodeID: nodeID, NodeAddr: h.node.NetAddr.String()}
	host := h.config.BootstrapServer
	id := h.DNAHash()
	url := fmt.Sprintf("http://%s/%s/%s", host, id.String(), nodeID)
	var b []byte
	b, err = json.Marshal(req)
	//var resp *http.Response
	if err == nil {
		_, err = http.Post(url, "application/json", bytes.NewBuffer(b))
	}
	return
}

func (h *Holochain) BSget() (err error) {
	host := h.config.BootstrapServer
	if host == "" {
		return
	}
	id := h.DNAHash()
	url := fmt.Sprintf("http://%s/%s", host, id.String())
	var resp *http.Response
	resp, err = http.Get(url)
	if err == nil {
		defer resp.Body.Close()
		var b []byte
		b, err = ioutil.ReadAll(resp.Body)
		if err == nil {
			var nodes []BSResp
			err = json.Unmarshal(b, &nodes)
			if err == nil {
				myNodeID := peer.IDB58Encode(h.node.HashAddr)
				for _, r := range nodes {
					var id peer.ID
					var addr ma.Multiaddr
					id, err = peer.IDB58Decode(r.Req.NodeID)
					if err == nil {
						addr, err = ma.NewMultiaddr(r.Req.NodeAddr)
						if err == nil {
							if myNodeID != r.Req.NodeID {
								h.dht.dlog.Logf("discovered peer: %s", r.Req.NodeID)
								h.node.Host.Peerstore().AddAddr(id, addr, pstore.PermanentAddrTTL)
								err = h.dht.UpdateGossiper(id, 0)

							}

						}
					}

				}
			}
		}
	}
	return
}
