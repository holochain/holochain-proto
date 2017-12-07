// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// gossip implements the gossip protocol for the distributed hash table

package holochain

import (
	"errors"
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	"github.com/tidwall/buntdb"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Put holds a put or link for gossiping
type Put struct {
	Idx int
	M   Message
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

// we also gossip about peers too, keeping lists of different peers e.g. blockedlist etc
type PeerListType string

const (
	BlockedList = "blockedlist"
)

type PeerRecord struct {
	ID      peer.ID
	Warrant string // evidence, reasons, documentation of why peer is in this list
}

type PeerList struct {
	Type    PeerListType
	Records []PeerRecord
}

var ErrDHTErrNoGossipersAvailable error = errors.New("no gossipers available")
var ErrDHTExpectedGossipReqInBody error = errors.New("expected gossip request")
var ErrNoSuchIdx error = errors.New("no such change index")

// incIdx adds a new index record to dht for gossiping later
func incIdx(tx *buntdb.Tx, m *Message) (index string, err error) {
	// if message is nil we can't record this for gossiping
	// this should only be the case for the DNA
	if m == nil {
		return
	}

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

		var decodedMessage interface{}
		err = ByteDecoder(b, decodedMessage)
	}
	_, _, err = tx.Set("idx:"+index, msg, nil)
	if err != nil {
		return
	}

	f, err := m.Fingerprint()
	if err != nil {
		return
	}
	_, _, err = tx.Set("f:"+f.String(), index, nil)
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

// GetIdxMessage returns the messages that causes the change at a given index
func (dht *DHT) GetIdxMessage(idx int) (msg Message, err error) {
	err = dht.db.View(func(tx *buntdb.Tx) error {
		msgStr, e := tx.Get(fmt.Sprintf("idx:%d", idx))
		if e == buntdb.ErrNotFound {
			return ErrNoSuchIdx
		}
		if e != nil {
			return e
		}
		e = ByteDecoder([]byte(msgStr), &msg)
		if err != nil {
			return e
		}
		return nil
	})
	return
}

//HaveFingerprint returns true if we have seen the given fingerprint
func (dht *DHT) HaveFingerprint(f Hash) (result bool, err error) {
	index, err := dht.GetFingerprint(f)
	if err == nil {
		result = index >= 0
	}
	return
}

// GetFingerprint returns the index that of the message that made a change or -1 if we don't have it
func (dht *DHT) GetFingerprint(f Hash) (index int, err error) {
	index = -1
	err = dht.db.View(func(tx *buntdb.Tx) error {
		idxStr, e := tx.Get("f:" + f.String())
		if e == buntdb.ErrNotFound {
			return nil
		}
		if e != nil {
			return e
		}
		index, e = strconv.Atoi(idxStr)
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
				p := Put{Idx: idx}
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
		sort.Slice(puts, func(i, j int) bool { return puts[i].Idx < puts[j].Idx })
		return err
	})
	return
}

// GetGossiper loads returns last known index of the gossiper, and adds them if not didn't exist before
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

func (dht *DHT) getGossipers() (glist []peer.ID, err error) {
	glist = make([]peer.ID, 0)

	err = dht.db.View(func(tx *buntdb.Tx) error {
		err = tx.Ascend("peer", func(key, value string) bool {
			x := strings.Split(key, ":")
			id, e := peer.IDB58Decode(x[1])
			if e != nil {
				return false
			}
			//			idx, _ := strconv.Atoi(value)
			glist = append(glist, id)
			return true
		})
		return nil
	})
	ns := dht.config.NeighborhoodSize
	if ns > 1 {
		size := len(glist)
		hlist := make([]Hash, size)
		for i := 0; i < size; i++ {
			h := HashFromPeerID(glist[i])
			hlist[i] = h
		}
		me := HashFromPeerID(dht.h.nodeID)

		hlist = SortByDistance(me, hlist)
		glist = make([]peer.ID, len(hlist))
		for i := 0; i < len(hlist); i++ {
			glist[i] = PeerIDFromHash(hlist[i])
		}
	}
	glist = dht.h.node.filterInactviePeers(glist, ns)
	return
}

// FindGossiper picks a random DHT node to gossip with
func (dht *DHT) FindGossiper() (g peer.ID, err error) {
	var glist []peer.ID
	glist, err = dht.getGossipers()
	if err != nil {
		return
	}
	if len(glist) == 0 {
		err = ErrDHTErrNoGossipersAvailable
	} else {
		g = glist[rand.Intn(len(glist))]
	}
	return
}

// AddGossiper adds a new gossiper to the gossiper store
func (dht *DHT) AddGossiper(id peer.ID) (err error) {
	// never add ourselves as a gossiper
	if id == dht.h.node.HashAddr {
		return
	}
	err = dht.updateGossiper(id, 0)
	return
}

// internal update gossiper function, assumes all checks have been made
func (dht *DHT) updateGossiper(id peer.ID, newIdx int) (err error) {
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		key := "peer:" + peer.IDB58Encode(id)
		idx, e := getIntVal(key, tx)
		if e != nil {
			return e
		}
		if newIdx < idx {
			return nil
		}
		sidx := fmt.Sprintf("%d", newIdx)
		_, _, err = tx.Set(key, sidx, nil)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

// UpdateGossiper updates a gossiper
func (dht *DHT) UpdateGossiper(id peer.ID, newIdx int) (err error) {
	if dht.h.node.IsBlocked(id) {
		dht.glog.Logf("gossiper %v on blocklist, deleting", id)
		dht.DeleteGossiper(id) // ignore error
		return
	}
	dht.glog.Logf("updating %v to %d", id, newIdx)
	err = dht.updateGossiper(id, newIdx)
	return
}

// DeleteGossiper removes a gossiper from the database
func (dht *DHT) DeleteGossiper(id peer.ID) (err error) {
	dht.glog.Logf("deleting %v", id)
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		key := "peer:" + peer.IDB58Encode(id)
		_, e := tx.Delete(key)
		return e
	})
	return
}

const (
	GossipBackPutDelay = 100 * time.Millisecond
)

// GossipReceiver implements the handler for the gossip protocol
func GossipReceiver(h *Holochain, m *Message) (response interface{}, err error) {
	dht := h.dht
	switch m.Type {
	case GOSSIP_REQUEST:
		dht.glog.Logf("GossipReceiver got: %v", m)
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
				dht.glog.Logf("we only have %d of %d from %v so gossiping back", idx, t.MyIdx, m.From)

				pi := h.node.host.Peerstore().PeerInfo(m.From)
				if len(pi.Addrs) == 0 {
					dht.glog.Logf("NO ADDRESSES FOR PEER:%v", pi)
				}

				// queue up a request to gossip back
				go func() {
					defer func() {
						if r := recover(); r != nil {
							// ignore writes past close
						}
					}()
					// but give them a chance to finish handling the response
					// from this request first so sleep a bit per put
					time.Sleep(GossipBackPutDelay * time.Duration(len(puts)))
					dht.gchan <- gossipWithReq{m.From}
				}()
			}

		default:
			err = ErrDHTExpectedGossipReqInBody
		}
	default:
		err = fmt.Errorf("message type %d not in holochain-gossip protocol", int(m.Type))
	}
	return
}

// gossipWith gossips with a peer asking for everything after since
func (dht *DHT) gossipWith(id peer.ID) (err error) {
	// prevent rentrance
	dht.glk.Lock()
	defer dht.glk.Unlock()

	dht.glog.Logf("starting gossipWith %v", id)
	defer func() {
		dht.glog.Logf("finish gossipWith %v, err=%v", id, err)
	}()

	var myIdx, yourIdx int
	myIdx, err = dht.GetIdx()
	if err != nil {
		return
	}

	yourIdx, err = dht.GetGossiper(id)
	if err != nil {
		return
	}

	var r interface{}
	msg := dht.h.node.NewMessage(GOSSIP_REQUEST, GossipReq{MyIdx: myIdx, YourIdx: yourIdx + 1})
	r, err = dht.h.Send(dht.h.node.ctx, GossipProtocol, id, msg, 0)
	if err != nil {
		return
	}

	gossip := r.(Gossip)
	puts := gossip.Puts

	// gossiper has more stuff that we new about before so update the gossipers status
	// and also run their puts
	count := len(puts)
	if count > 0 {
		dht.glog.Logf("queuing %d puts:\n%v", count, puts)
		var idx int
		for i, p := range puts {
			idx = i + yourIdx + 1
			// put the message into the gossip put handling queue so we can return quickly
			dht.gossipPuts <- p
		}
		err = dht.UpdateGossiper(id, idx)
	} else {
		dht.glog.Log("no new puts received")
	}
	return
}

// gossipPut handles a given put
func (dht *DHT) gossipPut(p Put) (err error) {
	f, e := p.M.Fingerprint()
	if e == nil {
		// dht.sources[p.M.From] = true
		// dht.fingerprints[f.String()[2:4]] = true
		dht.glog.Logf("PUT--%d (fingerprint: %v)", p.Idx, f)
		exists, e := dht.HaveFingerprint(f)
		if !exists && e == nil {
			dht.glog.Logf("PUT--%d calling ActionReceiver", p.Idx)
			r, e := ActionReceiver(dht.h, &p.M)
			dht.glog.Logf("PUT--%d ActionReceiver returned %v with err %v", p.Idx, r, e)
			if e != nil {
				// put receiver error so do what? probably nothing because
				// put will get retried
			}
		} else {
			if e == nil {
				dht.glog.Logf("already have fingerprint %v", f)
			} else {
				dht.glog.Logf("error in HaveFingerprint %v", e)
			}
		}

	} else {
		dht.glog.Logf("error calculating fingerprint for %v", p)
	}
	return
}

func handleGossipPut(dht *DHT) (stop bool, err error) {
	p, ok := <-dht.gossipPuts
	if !ok {
		stop = true
		return
	}
	err = dht.gossipPut(p)
	return
}

// gossip picks a random node in my neighborhood and sends gossips with it
func (dht *DHT) gossip() (err error) {

	var g peer.ID
	g, err = dht.FindGossiper()
	if err != nil {
		return
	}
	dht.gchan <- gossipWithReq{g}
	return
}

// GossipTask runs a gossip and logs any errors
func GossipTask(h *Holochain) {
	if h.dht != nil {
		if h.dht.gchan != nil {
			err := h.dht.gossip()
			if err != nil {
				h.dht.glog.Logf("error: %v", err)
			}
		}
	}
}

func handleGossipWith(dht *DHT) (stop bool, err error) {
	g, ok := <-dht.gchan
	if !ok {
		stop = true
		return
	}
	err = dht.gossipWith(g.id)
	return
}

func (dht *DHT) handleTillDone(errtext string, fn func(*DHT) (bool, error)) (err error) {
	var done bool
	for !done {
		dht.glog.Logf("HandleGossip%s: waiting for request", errtext)
		done, err = fn(dht)
		if err != nil {
			dht.glog.Logf("HandleGossip%s: got err: %v", errtext, err)
		}
	}
	dht.glog.Logf("HandleGossip%s: channel closed, stopping", errtext)
	return nil
}

// HandleGossipWiths waits on a channel for gossipWith requests
func (dht *DHT) HandleGossipWiths() (err error) {
	err = dht.handleTillDone("Withs", handleGossipWith)
	return
}

// HandleGossipPuts waits on a channel for gossip changes
func (dht *DHT) HandleGossipPuts() (err error) {
	err = dht.handleTillDone("Puts", handleGossipPut)
	return nil
}

// getList returns the peer list of the given type
func (dht *DHT) getList(listType PeerListType) (result PeerList, err error) {
	result.Type = listType
	result.Records = make([]PeerRecord, 0)
	err = dht.db.View(func(tx *buntdb.Tx) error {
		err = tx.Ascend("list", func(key, value string) bool {
			x := strings.Split(key, ":")

			if x[1] == string(listType) {
				pid, e := peer.IDB58Decode(x[2])
				if e != nil {
					return false
				}
				r := PeerRecord{ID: pid, Warrant: value}
				result.Records = append(result.Records, r)
			}
			return true
		})
		return nil
	})
	return
}

// addToList adds the peers to a list
func (dht *DHT) addToList(m *Message, list PeerList) (err error) {
	dht.dlog.Logf("addToList %s=>%v", list.Type, list.Records)
	err = dht.db.Update(func(tx *buntdb.Tx) error {
		_, err = incIdx(tx, m)
		if err != nil {
			return err
		}
		for _, r := range list.Records {
			k := peer.IDB58Encode(r.ID)
			_, _, err = tx.Set("list:"+string(list.Type)+":"+k, r.Warrant, nil)
			if err != nil {
				return err
			}
		}
		return err
	})
	return
}
