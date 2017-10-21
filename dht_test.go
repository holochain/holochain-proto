package holochain

import (
	"fmt"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/metacurrency/holochain/hash"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/tidwall/buntdb"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDHT(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	os.Remove(filepath.Join(h.DBPath(), DHTStoreFileName))

	Convey("It should initialize the DHT struct and data store", t, func() {
		So(FileExists(h.DBPath(), DHTStoreFileName), ShouldBeFalse)
		dht := NewDHT(h)
		So(FileExists(h.DBPath(), DHTStoreFileName), ShouldBeTrue)
		So(dht.h, ShouldEqual, h)
		So(dht.config, ShouldEqual, &h.nucleus.dna.DHTConfig)
	})
}

func TestSetupDHT(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	err := h.dht.SetupDHT()
	Convey("it should add the holochain ID to the DHT", t, func() {
		So(err, ShouldBeNil)
		ID := h.DNAHash()
		So(h.dht.exists(ID, StatusLive), ShouldBeNil)
		_, et, _, status, err := h.dht.get(h.dnaHash, StatusLive, GetMaskAll)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)
		So(et, ShouldEqual, DNAEntryType)

	})

	Convey("it should push the agent entry to the DHT at genesis time", t, func() {
		data, et, _, status, err := h.dht.get(h.agentHash, StatusLive, GetMaskAll)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)
		So(et, ShouldEqual, AgentEntryType)

		var e Entry
		e, _, _ = h.chain.GetEntry(h.agentHash)

		var b []byte
		b, _ = e.Marshal()

		So(string(data), ShouldEqual, string(b))
	})

	Convey("it should push the key to the DHT at genesis time", t, func() {
		keyHash, _ := NewHash(h.nodeIDStr)
		data, et, _, status, err := h.dht.get(keyHash, StatusLive, GetMaskAll)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)
		So(et, ShouldEqual, KeyEntryType)
		pubKey, err := ic.MarshalPublicKey(h.agent.PubKey())
		So(string(data), ShouldEqual, string(pubKey))

		data, et, _, status, err = h.dht.get(keyHash, StatusDefault, GetMaskDefault)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)

		So(string(data), ShouldEqual, string(pubKey))
	})
}

func TestPutGetModDel(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	dht := h.dht
	var id = h.nodeID
	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	var idx int
	Convey("It should store and retrieve", t, func() {
		err := dht.put(h.node.NewMessage(PUT_REQUEST, PutReq{H: hash}), "someType", hash, id, []byte("some value"), StatusLive)
		So(err, ShouldBeNil)
		idx, _ = dht.GetIdx()

		data, entryType, sources, status, err := dht.get(hash, StatusLive, GetMaskAll)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, StatusLive)
		So(sources[0], ShouldEqual, h.nodeIDStr)

		badhash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
		data, entryType, _, _, err = dht.get(badhash, StatusLive, GetMaskDefault)
		So(entryType, ShouldEqual, "")
		So(err, ShouldEqual, ErrHashNotFound)
	})

	Convey("mod should move the hash to the modified status and record replacedBy link", t, func() {
		m := h.node.NewMessage(MOD_REQUEST, hash)

		newhashStr := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh4"
		newhash, _ := NewHash(newhashStr)

		err := dht.mod(m, hash, newhash)
		So(err, ShouldBeNil)
		data, entryType, _, status, err := dht.get(hash, StatusAny, GetMaskAll)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, StatusModified)

		afterIdx, _ := dht.GetIdx()

		So(afterIdx-idx, ShouldEqual, 1)

		data, entryType, _, status, err = dht.get(hash, StatusLive, GetMaskDefault)
		So(err, ShouldEqual, ErrHashNotFound)

		data, entryType, _, status, err = dht.get(hash, StatusDefault, GetMaskDefault)
		So(err, ShouldEqual, ErrHashModified)
		// replaced by link gets returned in the data!!
		So(string(data), ShouldEqual, newhashStr)

		links, err := dht.getLinks(hash, SysTagReplacedBy, StatusLive)
		So(err, ShouldBeNil)
		So(len(links), ShouldEqual, 1)
		So(links[0].H, ShouldEqual, newhashStr)
	})

	Convey("del should move the hash to the deleted status", t, func() {
		m := h.node.NewMessage(DEL_REQUEST, hash)

		err := dht.del(m, hash)
		So(err, ShouldBeNil)

		data, entryType, _, status, err := dht.get(hash, StatusAny, GetMaskAll)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, StatusDeleted)

		afterIdx, _ := dht.GetIdx()

		So(afterIdx-idx, ShouldEqual, 2)

		data, entryType, _, status, err = dht.get(hash, StatusLive, GetMaskDefault)
		So(err, ShouldEqual, ErrHashNotFound)

		data, entryType, _, status, err = dht.get(hash, StatusDefault, GetMaskDefault)
		So(err, ShouldEqual, ErrHashDeleted)

	})
}

func TestLinking(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	err := h.dht.SetupDHT()
	dht := h.dht

	baseStr := "QmZcUPvPhD1Xvk6mwijYF8AfR3mG31S1YsEfHG4khrFPRr"
	base, err := NewHash(baseStr)
	if err != nil {
		panic(err)
	}
	linkingEntryHashStr := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3"
	linkingEntryHash, _ := NewHash(linkingEntryHashStr)
	linkHash1Str := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh1"
	linkHash1, _ := NewHash(linkHash1Str)
	linkHash2Str := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2"
	//linkHash2, _ := NewHash(linkHash2Str)
	Convey("It should fail if hash doesn't exist", t, func() {
		err := dht.putLink(nil, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldEqual, ErrHashNotFound)

		v, err := dht.getLinks(base, "tag foo", StatusLive)
		So(v, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	var id peer.ID
	err = dht.put(h.node.NewMessage(PUT_REQUEST, PutReq{H: base}), "someType", base, id, []byte("some value"), StatusLive)
	if err != nil {
		panic(err)
	}

	// the message doesn't actually matter for this test because it only gets used later in gossiping
	fakeMsg := h.node.NewMessage(LINK_REQUEST, LinkReq{Base: linkHash1, Links: linkingEntryHash})

	Convey("Low level should add linking events to buntdb", t, func() {
		err := dht.link(fakeMsg, baseStr, linkHash1Str, "link test", StatusLive)
		So(err, ShouldBeNil)
		err = dht.db.View(func(tx *buntdb.Tx) error {
			err = tx.Ascend("link", func(key, value string) bool {
				So(key, ShouldEqual, fmt.Sprintf(`link:%s:%s:link test`, baseStr, linkHash1Str))
				So(value, ShouldEqual, fmt.Sprintf(`[{"Status":%d,"Source":"%s","LinksEntry":"%s"}]`, StatusLive, h.nodeIDStr, linkingEntryHashStr))
				return true
			})
			return nil
		})

		err = dht.link(fakeMsg, baseStr, linkHash1Str, "link test", StatusDeleted)
		So(err, ShouldBeNil)
		err = dht.db.View(func(tx *buntdb.Tx) error {
			err = tx.Ascend("link", func(key, value string) bool {
				So(value, ShouldEqual, fmt.Sprintf(`[{"Status":%d,"Source":"%s","LinksEntry":"%s"},{"Status":%d,"Source":"%s","LinksEntry":"%s"}]`, StatusLive, h.nodeIDStr, linkingEntryHashStr, StatusDeleted, h.nodeIDStr, linkingEntryHashStr))
				return true
			})
			return nil
		})
	})

	Convey("It should store and retrieve links values on a base", t, func() {
		data, err := dht.getLinks(base, "tag foo", StatusLive)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "No links for tag foo")

		err = dht.putLink(fakeMsg, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)

		err = dht.putLink(fakeMsg, baseStr, linkHash2Str, "tag foo")
		So(err, ShouldBeNil)

		err = dht.putLink(fakeMsg, baseStr, linkHash1Str, "tag bar")
		So(err, ShouldBeNil)

		data, err = dht.getLinks(base, "tag foo", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 2)
		m := data[0]

		So(m.H, ShouldEqual, linkHash1Str)
		m = data[1]
		So(m.H, ShouldEqual, linkHash2Str)

		data, err = dht.getLinks(base, "tag bar", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)
		So(data[0].H, ShouldEqual, linkHash1Str)
	})

	Convey("It should store and retrieve a links source", t, func() {
		err = dht.putLink(fakeMsg, baseStr, linkHash1Str, "tag source")
		So(err, ShouldBeNil)

		data, err := dht.getLinks(base, "tag source", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)

		data, err = dht.getLinks(base, "tag source", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)
		So(data[0].Source, ShouldEqual, h.nodeIDStr)
	})

	Convey("It should work to put a link a second time", t, func() {
		err = dht.putLink(fakeMsg, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)
	})

	Convey("It should fail delete links non existent links bases and tags", t, func() {
		badHashStr := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqhX"

		err := dht.delLink(fakeMsg, badHashStr, linkHash1Str, "tag foo")
		So(err, ShouldEqual, ErrHashNotFound)
		err = dht.delLink(fakeMsg, baseStr, badHashStr, "tag foo")
		So(err, ShouldEqual, ErrLinkNotFound)
		err = dht.delLink(fakeMsg, baseStr, linkHash1Str, "tag baz")
		So(err, ShouldEqual, ErrLinkNotFound)
	})

	Convey("It should delete links", t, func() {
		err := dht.delLink(fakeMsg, baseStr, linkHash1Str, "tag bar")
		So(err, ShouldBeNil)
		data, err := dht.getLinks(base, "tag bar", StatusLive)
		So(err.Error(), ShouldEqual, "No links for tag bar")

		err = dht.delLink(fakeMsg, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)
		data, err = dht.getLinks(base, "tag foo", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)

		err = dht.delLink(fakeMsg, baseStr, linkHash2Str, "tag foo")
		So(err, ShouldBeNil)
		data, err = dht.getLinks(base, "tag foo", StatusLive)
		So(err.Error(), ShouldEqual, "No links for tag foo")
	})
}

func TestDHTSend(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("send GET_REQUEST message for non existent hash should get error", t, func() {
		msg := h.node.NewMessage(GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		_, err := h.dht.send(nil, h.node.HashAddr, msg)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "4"}
	_, hd, err := h.NewEntry(now, "evenNumbers", &e)
	if err != nil {
		panic(err)
	}

	// publish the entry data to the dht
	hash = hd.EntryLink
	Convey("after a handled PUT_REQUEST data should be stored in DHT", t, func() {
		msg := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash})
		r, err := h.dht.send(nil, h.node.HashAddr, msg)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeOK)
		hd, _ := h.chain.GetEntryHeader(hash)
		So(hd.EntryLink.Equal(&hash), ShouldBeTrue)
	})

	Convey("send GET_REQUEST message should return content", t, func() {
		msg := h.node.NewMessage(GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		r, err := h.dht.send(nil, h.node.HashAddr, msg)
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", e))
	})

	Convey("send GET_REQUEST message should return content of sys types", t, func() {
		msg := h.node.NewMessage(GET_REQUEST, GetReq{H: h.agentHash, StatusMask: StatusLive})
		r, err := h.dht.send(nil, h.nodeID, msg)
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		ae, _ := h.agent.AgentEntry(nil)
		So(fmt.Sprintf("%v", resp.Entry.Content()), ShouldEqual, fmt.Sprintf("%v", ae))

		msg = h.node.NewMessage(GET_REQUEST, GetReq{H: HashFromPeerID(h.nodeID), StatusMask: StatusLive})
		r, err = h.dht.send(nil, h.nodeID, msg)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry.Content()), ShouldEqual, fmt.Sprintf("%v", ae.PublicKey))

		// for now this is an error because we presume everyone has the DNA.
		// once we implement dna changes, this needs to be changed
		msg = h.node.NewMessage(GET_REQUEST, GetReq{H: h.dnaHash, StatusMask: StatusLive})
		r, err = h.dht.send(nil, h.nodeID, msg)
		So(err, ShouldBeError)

	})
}

func TestDHTQueryGet(t *testing.T) {
	nodesCount := 6
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()

	h := mt.nodes[0]

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "4"}
	_, hd, err := h.NewEntry(now, "evenNumbers", &e)
	if err != nil {
		panic(err)
	}

	/*for i := 0; i < nodesCount; i++ {
		fmt.Printf("node%d:%v\n", i, mt.nodes[i].node.HashAddr.Pretty()[2:6])
	}*/

	// publish the entry data to local DHT node (0)
	hash := hd.EntryLink
	msg := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash})
	_, err = h.dht.send(nil, h.node.HashAddr, msg)
	if err != nil {
		panic(err)
	}

	ringConnect(t, mt.ctx, mt.nodes, nodesCount)

	// pick a distant node that has to do some of the recursive lookups to get back to node 0.
	Convey("Kademlia GET_REQUEST should return content", t, func() {
		h2 := mt.nodes[nodesCount-2]
		r, err := h2.dht.Query(hash, GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", e))
	})
}

func TestDHTKadPut(t *testing.T) {
	nodesCount := 6
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()

	h := mt.nodes[0]

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "4"}
	_, hd, err := h.NewEntry(now, "evenNumbers", &e)
	if err != nil {
		panic(err)
	}
	hash := hd.EntryLink

	/*
		for i := 0; i < nodesCount; i++ {
			fmt.Printf("node%d:%v\n", i, mt.nodes[i].node.HashAddr.Pretty()[2:6])
		}
		//node0:NnRV
		//node1:UfY4
		//node2:YA62
		//node3:S4BF
		//node4:W4He
		//node5:dxxu

		starConnect(t, mt.ctx, mt.nodes, nodesCount)
		// get closest peers in the routing table
		rtp := h.node.routingTable.NearestPeers(hash, AlphaValue)
		fmt.Printf("CLOSE:%v\n", rtp)

		//[<peer.ID S4BFeT> <peer.ID W4HeEG> <peer.ID UfY4We>]
		//i.e 3,4,1
	*/

	ringConnect(t, mt.ctx, mt.nodes, nodesCount)

	Convey("Kademlia PUT_REQUEST should put the hash to its closet node even if we don't know about it yet", t, func() {
		rtp := h.node.routingTable.NearestPeers(hash, AlphaValue)
		// check that our routing table doesn't contain closest node yet
		So(fmt.Sprintf("%v", rtp), ShouldEqual, "[<peer.ID UfY4We> <peer.ID dxxuES>]")
		err := h.dht.Change(hash, PUT_REQUEST, PutReq{H: hash})
		So(err, ShouldBeNil)
		rtp = h.node.routingTable.NearestPeers(hash, AlphaValue)
		// routing table should be updated
		So(fmt.Sprintf("%v", rtp), ShouldEqual, "[<peer.ID S4BFeT> <peer.ID W4HeEG> <peer.ID UfY4We>]")
		// and get from node should get the value
		msg := h.node.NewMessage(GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		r, err := h.dht.send(nil, mt.nodes[3].nodeID, msg)
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", e))
	})
}

func TestActionReceiver(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("PUT_REQUEST should fail if body isn't a hash", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, "foo")
		_, err := ActionReceiver(h, m)
		So(err.Error(), ShouldEqual, "Unexpected request body type 'string' in put request, expecting holochain.PutReq")
	})

	Convey("LINK_REQUEST should fail if body not a good linking request", t, func() {
		m := h.node.NewMessage(LINK_REQUEST, "foo")
		_, err := ActionReceiver(h, m)
		So(err.Error(), ShouldEqual, "Unexpected request body type 'string' in link request, expecting holochain.LinkReq")
	})

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "124"}
	_, hd, _ := h.NewEntry(now, "evenNumbers", &e)
	hash := hd.EntryLink

	Convey("PUT_REQUEST should queue a valid message", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash})
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeOK)
	})

	Convey("GET_REQUEST should return the requested values", t, func() {
		m := h.node.NewMessage(GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", e))

		m = h.node.NewMessage(GET_REQUEST, GetReq{H: hash, GetMask: GetMaskEntryType})
		r, err = ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(resp.Entry.C, ShouldBeNil)
		So(resp.EntryType, ShouldEqual, "evenNumbers")

		m = h.node.NewMessage(GET_REQUEST, GetReq{H: hash, GetMask: GetMaskEntry + GetMaskEntryType})
		r, err = ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", e))
		So(resp.EntryType, ShouldEqual, "evenNumbers")

		m = h.node.NewMessage(GET_REQUEST, GetReq{H: hash, GetMask: GetMaskSources})
		r, err = ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(resp.Entry.C, ShouldBeNil)
		So(fmt.Sprintf("%v", resp.Sources), ShouldEqual, fmt.Sprintf("[%v]", h.nodeIDStr))
		So(resp.EntryType, ShouldEqual, "")

		m = h.node.NewMessage(GET_REQUEST, GetReq{H: hash, GetMask: GetMaskEntry + GetMaskSources})
		r, err = ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", e))
		So(fmt.Sprintf("%v", resp.Sources), ShouldEqual, fmt.Sprintf("[%v]", h.nodeIDStr))
		So(resp.EntryType, ShouldEqual, "")
	})

	someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
	e = GobEntry{C: someData}
	_, hd, _ = h.NewEntry(now, "profile", &e)
	profileHash := hd.EntryLink

	le := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"},{"Base":"%s","Link":"%s","Tag":"3stars"}]}`, hash.String(), profileHash.String(), hash.String(), h.agentHash)}
	_, lhd, _ := h.NewEntry(time.Now(), "rating", &le)

	Convey("LINK_REQUEST should store links", t, func() {
		lr := LinkReq{Base: hash, Links: lhd.EntryLink}
		m := h.node.NewMessage(LINK_REQUEST, lr)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeOK)

		// check that it got put
		meta, err := h.dht.getLinks(hash, "4stars", StatusLive)
		So(err, ShouldBeNil)
		So(meta[0].H, ShouldEqual, hd.EntryLink.String())
	})

	e2 := GobEntry{C: "322"}
	hash2, _ := e2.Sum(h.hashSpec)

	e3 := GobEntry{C: "324"}
	hash3, _ := e3.Sum(h.hashSpec)

	Convey("LINK_REQUEST of unknown hash should get queued for retry", t, func() {
		lr := LinkReq{Base: hash2, Links: hash3}
		m := h.node.NewMessage(LINK_REQUEST, lr)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeUnknownHashQueuedForRetry)
		So(len(h.dht.retryQueue), ShouldEqual, 1)
		<-h.dht.retryQueue // unload the queue
	})

	Convey("GETLINK_REQUEST should retrieve link values", t, func() {
		mq := LinkQuery{Base: hash, T: "4stars"}
		m := h.node.NewMessage(GETLINK_REQUEST, mq)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		results := r.(*LinkQueryResp)
		So(results.Links[0].H, ShouldEqual, hd.EntryLink.String())
		So(results.Links[0].T, ShouldEqual, "")
	})

	Convey("GETLINK_REQUEST with empty tag should retrieve all linked values", t, func() {
		mq := LinkQuery{Base: hash, T: ""}
		m := h.node.NewMessage(GETLINK_REQUEST, mq)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		results := r.(*LinkQueryResp)
		var l4star, l3star TaggedHash
		// could come back in any order...
		if results.Links[0].T == "4stars" {
			l4star = results.Links[0]
			l3star = results.Links[1]

		} else {
			l4star = results.Links[1]
			l3star = results.Links[0]
		}
		So(l3star.H, ShouldEqual, h.agentHash.String())
		So(l3star.T, ShouldEqual, "3stars")
		So(l4star.H, ShouldEqual, hd.EntryLink.String())
		So(l4star.T, ShouldEqual, "4stars")
	})

	Convey("GOSSIP_REQUEST should request and advertise data by idx", t, func() {
		g := GossipReq{MyIdx: 1, YourIdx: 2}
		m := h.node.NewMessage(GOSSIP_REQUEST, g)
		r, err := GossipReceiver(h, m)
		So(err, ShouldBeNil)
		gr := r.(Gossip)
		So(len(gr.Puts), ShouldEqual, 4)
	})

	le2 := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars","LinkAction":"%s"}]}`, hash.String(), profileHash.String(), DelAction)}
	_, lhd2, _ := h.NewEntry(time.Now(), "rating", &le2)

	Convey("LINK_REQUEST with del type should mark a link as deleted", t, func() {
		lr := LinkReq{Base: hash, Links: lhd2.EntryLink}
		m := h.node.NewMessage(LINK_REQUEST, lr)
		r, err := ActionReceiver(h, m)
		So(r, ShouldEqual, DHTChangeOK)

		_, err = h.dht.getLinks(hash, "4stars", StatusLive)
		So(err.Error(), ShouldEqual, "No links for 4stars")

		results, err := h.dht.getLinks(hash, "4stars", StatusDeleted)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 1)
	})

	Convey("GETLINK_REQUEST with mask option should retrieve deleted link values", t, func() {
		mq := LinkQuery{Base: hash, T: "4stars", StatusMask: StatusDeleted}
		m := h.node.NewMessage(GETLINK_REQUEST, mq)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		results := r.(*LinkQueryResp)
		So(results.Links[0].H, ShouldEqual, hd.EntryLink.String())
	})

	Convey("MOD_REQUEST of unknown hash should get queued for retry", t, func() {
		req := ModReq{H: hash2, N: hash3}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeUnknownHashQueuedForRetry)
		So(len(h.dht.retryQueue), ShouldEqual, 1)
		<-h.dht.retryQueue // unload the queue
	})

	// put a second entry to DHT
	h.NewEntry(now, "evenNumbers", &e2)
	m2 := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash2})
	ActionReceiver(h, m2)

	Convey("MOD_REQUEST should set hash to modified", t, func() {
		req := ModReq{H: hash, N: hash2}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeOK)
	})

	Convey("DELETE_REQUEST should set status of hash to deleted", t, func() {
		entry := DelEntry{Hash: hash2, Message: "expired"}
		a := NewDelAction("evenNumbers", entry)
		_, _, entryHash, err := h.doCommit(a, &StatusChange{Action: DelAction, Hash: hash2})

		m := h.node.NewMessage(DEL_REQUEST, DelReq{H: hash2, By: entryHash})
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeOK)

		data, entryType, _, status, _ := h.dht.get(hash2, StatusAny, GetMaskAll)
		var e GobEntry
		e.Unmarshal(data)
		So(e.C, ShouldEqual, "322")
		So(entryType, ShouldEqual, "evenNumbers")
		So(status, ShouldEqual, StatusDeleted)
	})

	Convey("DELETE_REQUEST of unknown hash should get queued for retry", t, func() {
		req := DelReq{H: hash3, By: hash3}
		m := h.node.NewMessage(DEL_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeUnknownHashQueuedForRetry)
		So(len(h.dht.retryQueue), ShouldEqual, 1)
	})

	Convey("LISTADD_REQUEST with bad warrant should return error", t, func() {
		pid, _ := makePeer("testPeer")
		m := h.node.NewMessage(LISTADD_REQUEST,
			ListAddReq{
				ListType:    BlockedList,
				Peers:       []string{peer.IDB58Encode(pid)},
				WarrantType: SelfRevocationType,
				Warrant:     []byte("bad warrant!"),
			})
		_, err := ActionReceiver(h, m)
		So(err.Error(), ShouldEqual, "List add request rejected on warrant failure: unable to decode warrant (invalid character 'b' looking for beginning of value)")
	})

	Convey("LISTADD_REQUEST with warrant out of context should return error", t, func() {
		pid, oldPrivKey := makePeer("testPeer")
		_, newPrivKey := makePeer("peer1")
		revocation, _ := NewSelfRevocation(oldPrivKey, newPrivKey, []byte("extra data"))
		w, _ := NewSelfRevocationWarrant(revocation)
		data, _ := w.Encode()
		m := h.node.NewMessage(LISTADD_REQUEST,
			ListAddReq{
				ListType:    BlockedList,
				Peers:       []string{peer.IDB58Encode(pid)},
				WarrantType: SelfRevocationType,
				Warrant:     data,
			})
		_, err := ActionReceiver(h, m)
		So(err.Error(), ShouldEqual, "List add request rejected on warrant failure: expected old key to be modified on DHT")

	})

	/*
		getting a good warrant without also having already had the addToList happen is hard,
		 so not quite sure how to test this
				Convey("LISTADD_REQUEST with good warrant should add to list", t, func() {
					pid, oldPrivKey := makePeer("testPeer")
					_, newPrivKey := makePeer("peer1")
					revocation, _ := NewSelfRevocation(oldPrivKey, newPrivKey, []byte("extra data"))
					w, _ := NewSelfRevocationWarrant(revocation)
					data, _ := w.Encode()
					m := h.node.NewMessage(LISTADD_REQUEST,
						ListAddReq{
							ListType:    BlockedList,
							Peers:       []string{peer.IDB58Encode(pid)},
							WarrantType: SelfRevocationType,
							Warrant:     data,
						})
					r, err := ActionReceiver(h, m)
					So(err, ShouldBeNil)
					So(r, ShouldEqual, DHTChangeOK)

					peerList, err := h.dht.getList(BlockedList)
					So(err, ShouldBeNil)
					So(len(peerList.Records), ShouldEqual, 1)
					So(peerList.Records[0].ID, ShouldEqual, pid)
				})
	*/

}

func TestDHTDump(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("dht dump of index 1 should show the agent put", t, func() {
		msg, _ := h.dht.GetIdxMessage(1)
		f, _ := msg.Fingerprint()
		msgStr := msg.String()

		str, err := h.dht.DumpIdx(1)
		So(err, ShouldBeNil)

		So(strings.Index(str, fmt.Sprintf("MSG (fingerprint %v)", f)) >= 0, ShouldBeTrue)
		So(strings.Index(str, msgStr) >= 0, ShouldBeTrue)
	})

	Convey("dht dump of index 99 should return err", t, func() {
		_, err := h.dht.DumpIdx(99)
		So(err.Error(), ShouldEqual, "no such change index")
	})

	reviewHash := commit(h, "review", "this is my bogus review of the user")
	commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, h.nodeIDStr, reviewHash.String()))

	Convey("dht.String() should produce human readable DHT", t, func() {
		dump := h.dht.String()
		So(dump, ShouldContainSubstring, "DHT changes: 4")
		d, _ := h.dht.DumpIdx(1)
		So(dump, ShouldContainSubstring, d)
		d, _ = h.dht.DumpIdx(2)
		So(dump, ShouldContainSubstring, d)

		So(dump, ShouldContainSubstring, "DHT entries:")
		So(dump, ShouldContainSubstring, fmt.Sprintf("Hash--%s (status 1)", h.nodeIDStr))
		pk, _ := h.agent.PubKey().Bytes()
		So(dump, ShouldContainSubstring, fmt.Sprintf("Value: %s", string(pk)))
		So(dump, ShouldContainSubstring, fmt.Sprintf("Sources: %s", h.nodeIDStr))

		So(dump, ShouldContainSubstring, fmt.Sprintf("Linked to: %s with tag %s", reviewHash, "4stars"))

	})
}

func TestDHTRetry(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	d1 := `{"firstName":"Zippy","lastName":"Pinhead"}`
	e := GobEntry{C: d1}
	hash, _ := e.Sum(h.hashSpec)
	d2 := `{"firstName":"Zerbina","lastName":"Pinhead"}`
	e2 := GobEntry{C: d2}
	hash2, _ := e2.Sum(h.hashSpec)

	Convey("it should make a change after some retries", t, func() {
		req := ModReq{H: hash, N: hash2}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeUnknownHashQueuedForRetry)

		// pause for a few retires
		h.node.retrying = h.TaskTicker(time.Millisecond*10, RetryTask)
		time.Sleep(time.Millisecond * 25)

		// add the entries and get them into the DHT
		h.NewEntry(time.Now(), "profile", &e)
		h.NewEntry(time.Now(), "profile", &e2)
		m = h.node.NewMessage(PUT_REQUEST, PutReq{H: hash})
		err = h.dht.put(m, "profile", hash, h.nodeID, []byte(d1), StatusLive)
		So(err, ShouldBeNil)
		m = h.node.NewMessage(PUT_REQUEST, PutReq{H: hash2})
		err = h.dht.put(m, "profile", hash2, h.nodeID, []byte(d2), StatusLive)
		So(err, ShouldBeNil)

		_, _, _, status, _ := h.dht.get(hash, StatusAny, GetMaskAll)
		So(status, ShouldEqual, StatusLive)

		// wait for next retry
		time.Sleep(time.Millisecond * 40)

		_, _, _, status, _ = h.dht.get(hash, StatusAny, GetMaskAll)
		So(status, ShouldEqual, StatusModified)

		// stop retrying for next test
		stop := h.node.retrying
		h.node.retrying = nil
		stop <- true

	})

	Convey("retries should be limited", t, func() {
		e3 := GobEntry{C: `{"firstName":"Zappy","lastName":"Pinhead"}`}
		hash3, _ := e3.Sum(h.hashSpec)
		e4 := GobEntry{C: `{"firstName":"Zuppy","lastName":"Pinhead"}`}
		hash4, _ := e4.Sum(h.hashSpec)
		req := ModReq{H: hash3, N: hash4}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeUnknownHashQueuedForRetry)
		So(len(h.dht.retryQueue), ShouldEqual, 1)

		interval := time.Millisecond * 10
		h.node.retrying = h.TaskTicker(interval, RetryTask)
		time.Sleep(interval * (MaxRetries + 2))
		So(len(h.dht.retryQueue), ShouldEqual, 0)
	})
}

func TestDHTMultiNode(t *testing.T) {
	nodesCount := 10
	mt := setupMultiNodeTesting(nodesCount)
	defer mt.cleanupMultiNodeTesting()
	nodes := mt.nodes

	ringConnectMutual(t, mt.ctx, mt.nodes, nodesCount)
	var connections int
	Convey("each node should be able to get the key of others", t, func() {
		for i := 0; i < nodesCount; i++ {
			h1 := nodes[i]
			for j := 0; j < nodesCount; j++ {
				h2 := nodes[j]
				options := GetOptions{StatusMask: StatusDefault}
				req := GetReq{H: HashFromPeerID(h1.nodeID), StatusMask: options.StatusMask, GetMask: options.GetMask}
				response, err := NewGetAction(req, &options).Do(h2)
				if err != nil {
					//fmt.Printf("FAIL   : %v couldn't get from %v\n", h2.nodeID, h1.nodeID)
				} else {
					pk, _ := h1.agent.PubKey().Bytes()
					if fmt.Sprintf("%v", response) == fmt.Sprintf("{{%v}  [] }", pk) {
						connections += 1
						//	fmt.Printf("SUCCESS: %v got from          %v\n", h2.nodeID, h1.nodeID)
					}
				}
			}
		}
		So(connections, ShouldEqual, nodesCount*nodesCount)
	})

	hashes := []Hash{}
	// add a bunch of data and links to that data on the key
	for i := 0; i < nodesCount; i++ {
		h := nodes[i]
		hash := commit(h, "review", fmt.Sprintf("this statement by node %d (%v)", i, h.nodeID))
		commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"statement"}]}`, h.nodeIDStr, hash.String()))
		hashes = append(hashes, hash)
	}

	Convey("each node should be able to get statements form the other nodes including self", t, func() {
		for i := 0; i < nodesCount; i++ {
			h1 := nodes[i]
			for j := 0; j < nodesCount; j++ {
				h2 := nodes[j]
				options := GetLinksOptions{Load: true, StatusMask: StatusLive}
				response, err := NewGetLinksAction(
					&LinkQuery{
						Base:       HashFromPeerID(h1.nodeID),
						T:          "statement",
						StatusMask: options.StatusMask,
					}, &options).Do(h2)
				So(err, ShouldBeNil)
				So(fmt.Sprintf("%v", response), ShouldEqual, fmt.Sprintf("&{[{%v this statement by node %d (%v) review  %s}]}", hashes[i], i, h1.nodeID, h1.nodeID.Pretty()))
			}
		}
	})
}
