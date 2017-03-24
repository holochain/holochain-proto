// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// gossip implements the gossip protocol for the distributed hash table

package holochain

import (
	"errors"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/tidwall/buntdb"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Put holds a put or link for gossiping
type Put struct {
	M Message
}

// Gossip holds a gossip message
type Gossip struct {
	Puts []Put
}

// GossipReq holds a gossip request
type GossipReq struct {
	MyIdx   int
	YourIdx int
}

// Gossiper holds data about a gossiper
type Gossiper struct {
	Id       peer.ID
	Idx      int
	LastSeen time.Time
}

var ErrDHTErrNoGossipersAvailable error = errors.New("no gossipers available")
var ErrDHTExpectedGossipReqInBody error = errors.New("expected gossip request")

// incIdx adds a new index record to dht for gossiping later
func incIdx(tx *buntdb.Tx, m *Message) (index string, err error) {
	var idx int
	idx, err = getIntVal("_idx", tx)
	if err != nil {
		return
	}
	idx++
	index = fmt.Sprintf("%d", idx)
	_, _, err = tx.Set("_idx", index, nil)
	if err != nil {
		return
	}

	var msg string

	if m != nil {
		var b []byte
		b, err = ByteEncoder(m)
		if err != nil {
			return
		}
		msg = string(b)
	}
	_, _, err = tx.Set("idx:"+index, msg, nil)
	if err != nil {
		return
	}
	return
}

// getIntVal returns an integer value at a given key, and assumes the value 0 if the key doesn't exist
func getIntVal(key string, tx *buntdb.Tx) (idx int, err error) {
	var val string
	val, err = tx.Get(key)
	if err == buntdb.ErrNotFound {
		err = nil
	} else if err != nil {
		return
	} else {
		idx, err = strconv.Atoi(val)
		if err != nil {
			return
		}
	}
	return
}

// GetIdx returns the current put index for gossip
func (dht *DHT) GetIdx() (idx int, err error) {
	err = dht.db.View(func(tx *buntdb.Tx) error {
		var e error
		idx, e = getIntVal("_idx", tx)
		if e != nil {
			return e
		}
		return nil
	})
	return
}

// GetPuts returns a list of puts after the given index
func (dht *DHT) GetPuts(since int) (puts []Put, err error) {
	puts = make([]Put, 0)
	err = dht.db.View(func(tx *buntdb.Tx) error {
		err = tx.AscendGreaterOrEqual("idx", string(since), func(key, value string) bool {
			x := strings.Split(key, ":")
			idx, _ := strconv.Atoi(x[1])
			if idx >= since {
				var p Put
				if value != "" {
					err := ByteDecoder([]byte(value), &p.M)
					if err != nil {
						return false
					}
				}
				puts = append(puts, p)
			}
			return true
		})
		return err
	})
	return
}

// GetGossiper picks a random DHT node to gossip with
func (dht *DHT) GetGossiper(id peer.ID) (idx int, err error) {
	key := "peer:" + peer.IDB58Encode(id)
	err = dht.db.View(func(tx *buntdb.Tx) error {
		var e error
		idx, e = getIntVal(key, tx)
		if e != nil {
			return e
		}
		return nil
	})
	return
}

// FindGossiper picks a random DHT node to gossip with
func (dht *DHT) FindGossiper() (g *Gossiper, err error) {
	glist := make([]Gossiper, 0)

	err = dht.db.View(func(tx *buntdb.Tx) error {
		err = tx.Ascend("peer", func(key, value string) bool {
			x := strings.Split(key, ":")
			id, e := peer.IDB58Decode(x[1])
			if e != nil {
				return false
			}
			idx, e := strconv.Atoi(value)
			g := Gossiper{Id: id, Idx: idx}
			glist = append(glist, g)
			return true
		})
		return nil
	})

	if len(glist) == 0 {
		err = ErrDHTErrNoGossipersAvailable
	} else {
		g = &glist[rand.Intn(len(glist))]
	}
	return
}

// UpdateGossiper updates a gossiper
func (dht *DHT) UpdateGossiper(id peer.ID, count int) (err error) {
	dht.glog.Logf("updaing %v with %d", id, count)
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		key := "peer:" + peer.IDB58Encode(id)
		idx, e := getIntVal(key, tx)
		if e != nil {
			return e
		}
		sidx := fmt.Sprintf("%d", idx+count)
		_, _, err = tx.Set(key, sidx, nil)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

func GossipReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	dht := h.dht
	switch m.Type {
	case GOSSIP_REQUEST:
		dht.glog.Logf("GossipReceiver got GOSSIP_REQUEST: %v", m)
		switch t := m.Body.(type) {
		case GossipReq:
			dht.glog.Logf("%v wants my puts since %d and is at %d", m.From, t.YourIdx, t.MyIdx)

			// give the gossiper what they want
			var puts []Put
			puts, err = h.dht.GetPuts(t.YourIdx)
			g := Gossip{Puts: puts}
			response = g

			// check to see what we know they said, and if our record is less
			// that where they are currently at, gossip back
			idx, e := h.dht.GetGossiper(m.From)
			if e == nil && idx < t.MyIdx {
				dht.glog.Logf("we only have %d from %v so gossiping back", idx, m.From)
				go func() {
					e := h.dht.gossipWith(m.From, idx)
					if e != nil {
						dht.glog.Logf("gossip back returned error: %v", e)
					}
				}()
			}

		default:
			err = ErrDHTExpectedGossipReqInBody
		}
	}
	return
}

// gossipWith gossips with an peer asking for everything after since
func (dht *DHT) gossipWith(id peer.ID, after int) (err error) {
	dht.glog.Logf("with %v", id)

	var myIdx int
	myIdx, err = dht.GetIdx()
	if err != nil {
		return
	}

	var r interface{}
	r, err = dht.h.Send(GossipProtocol, id, GOSSIP_REQUEST, GossipReq{MyIdx: myIdx, YourIdx: after + 1})
	if err != nil {
		return
	}

	gossip := r.(Gossip)
	puts := gossip.Puts
	dht.glog.Logf("received puts: %v", puts)

	// gossiper has more stuff that we new about before so update the gossipers status
	// and also run their puts
	if len(puts) > 0 {
		err = dht.UpdateGossiper(id, len(puts))
		for _, p := range puts {
			dht.glog.Log("running puts")
			DHTReceiver(dht.h, &p.M)
		}
	}
	return
}

// gossip picks a random node in my neighborhood and sends gossips with it
func (dht *DHT) gossip() (err error) {

	var g *Gossiper
	g, err = dht.FindGossiper()
	if err != nil {
		return
	}

	err = dht.gossipWith(g.Id, g.Idx)
	return
}

// Gossip gossips every interval
func (dht *DHT) Gossip(interval time.Duration) {
	dht.gossiping = true
	for dht.gossiping {
		err := dht.gossip()
		if err != nil {
			dht.glog.Logf("error: %v", err)
		}
		time.Sleep(interval)
	}
}
