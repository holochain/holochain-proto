// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements bootstrap server access

package holochain

import (
	"bytes"
	"encoding/json"
	"fmt"
	//ma "gx/ipfs/QmSWLfmj5frN9xVLMMN846dMDriy5wN5jeghUm7aTW3DAG/go-multiaddr"
	peer "github.com/libp2p/go-libp2p-peer"
	"net/http"
)

type BSReq struct {
	Version  int
	NodeID   string
	NodeAddr string
}

func (h *Holochain) BSpost() {
	nodeID := peer.IDB58Encode(h.node.HashAddr)
	req := BSReq{Version: 1, NodeID: nodeID, NodeAddr: h.node.NetAddr.String()}
	host := h.config.BootstrapServer
	id, _ := h.ID()
	url := fmt.Sprintf("http://%s/%s/%s", host, id.String(), nodeID)
	log.Infof("posting to:%s", url)
	b, err := json.Marshal(req)
	var resp *http.Response
	if err == nil {
		resp, err = http.Post(url, "application/json", bytes.NewBuffer(b))
	}
	if err != nil {
		log.Info(err.Error())
	}
	log.Infof("Response: %v", resp)

}
