package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewDHT(t *testing.T) {
	d := SetupTestDir()
	defer CleanupTestDir(d)
	var h Holochain
	h.rootPath = d
	os.MkdirAll(h.DBPath(), os.ModePerm)

	dht := NewDHT(&h)
	Convey("It should initialize the DHT struct", t, func() {
		So(dht.h, ShouldEqual, &h)
		So(fileExists(h.DBPath()+"/"+DHTStoreFileName), ShouldBeTrue)
	})
}

func TestSetupDHT(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

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
		So(string(data), ShouldEqual, string([]byte(h.nodeID)))

		data, et, _, status, err = h.dht.get(keyHash, StatusDefault, GetMaskDefault)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)
		So(string(data), ShouldEqual, string([]byte(h.nodeID)))
	})
}

func TestPutGetModDel(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

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

		links, err := dht.getLink(hash, SysTagReplacedBy, StatusLive)
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
	defer CleanupTestDir(d)

	err := h.dht.SetupDHT()
	dht := h.dht

	baseStr := "QmZcUPvPhD1Xvk6mwijYF8AfR3mG31S1YsEfHG4khrFPRr"
	base, err := NewHash(baseStr)
	if err != nil {
		panic(err)
	}
	linkHash1Str := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh1"
	//linkHash1, _ := NewHash(linkHash1Str)
	linkHash2Str := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2"
	//linkHash2, _ := NewHash(linkHash2Str)
	Convey("It should fail if hash doesn't exist", t, func() {
		err := dht.putLink(nil, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldEqual, ErrHashNotFound)

		v, err := dht.getLink(base, "tag foo", StatusLive)
		So(v, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	var id peer.ID
	err = dht.put(h.node.NewMessage(PUT_REQUEST, PutReq{H: base}), "someType", base, id, []byte("some value"), StatusLive)
	if err != nil {
		panic(err)
	}

	// the message doesn't actually matter for this test because it only gets used later in gossiping
	fakeMsg := h.node.NewMessage(LINK_REQUEST, LinkReq{})

	Convey("It should store and retrieve links values on a base", t, func() {
		data, err := dht.getLink(base, "tag foo", StatusLive)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "No links for tag foo")

		err = dht.putLink(fakeMsg, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)

		err = dht.putLink(fakeMsg, baseStr, linkHash2Str, "tag foo")
		So(err, ShouldBeNil)

		err = dht.putLink(fakeMsg, baseStr, linkHash1Str, "tag bar")
		So(err, ShouldBeNil)

		data, err = dht.getLink(base, "tag foo", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 2)
		m := data[0]

		So(m.H, ShouldEqual, linkHash1Str)
		m = data[1]
		So(m.H, ShouldEqual, linkHash2Str)

		data, err = dht.getLink(base, "tag bar", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)
		So(data[0].H, ShouldEqual, linkHash1Str)
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
		data, err := dht.getLink(base, "tag bar", StatusLive)
		So(err.Error(), ShouldEqual, "No links for tag bar")

		err = dht.delLink(fakeMsg, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)
		data, err = dht.getLink(base, "tag foo", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)

		err = dht.delLink(fakeMsg, baseStr, linkHash2Str, "tag foo")
		So(err, ShouldBeNil)
		data, err = dht.getLink(base, "tag foo", StatusLive)
		So(err.Error(), ShouldEqual, "No links for tag foo")
	})
}

func TestFindNodeForHash(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	Convey("It should find a node", t, func() {

		// for now the node it finds is ourself for any hash because we haven't implemented
		// anything about neighborhoods or other nodes...
		hash, err := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		if err != nil {
			panic(err)
		}
		node, err := h.dht.FindNodeForHash(hash)
		So(err, ShouldBeNil)
		So(node.HashAddr.Pretty(), ShouldEqual, h.nodeID.Pretty())
	})
}

func TestSend(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	agent := h.Agent().(*LibP2PAgent)
	node, err := NewNode("/ip4/127.0.0.1/tcp/1234", agent)
	if err != nil {
		panic(err)
	}
	defer node.Close()

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("send GET_REQUEST message for non existent hash should get error", t, func() {
		_, err := h.dht.send(node.HashAddr, GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
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
		r, err := h.dht.send(node.HashAddr, PUT_REQUEST, PutReq{H: hash})
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
		h.dht.simHandleChangeReqs()
		hd, _ := h.chain.GetEntryHeader(hash)
		So(hd.EntryLink.Equal(&hash), ShouldBeTrue)
	})

	Convey("send GET_REQUEST message should return content", t, func() {
		r, err := h.dht.send(node.HashAddr, GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", &e))
	})
}

func TestActionReceiver(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

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

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("LINK_REQUEST should fail if hash doesn't exist", t, func() {
		me := LinkReq{Base: hash, Links: hash}
		m := h.node.NewMessage(LINK_REQUEST, me)
		_, err := ActionReceiver(h, m)
		So(err.Error(), ShouldEqual, "hash not found")
	})

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "124"}
	_, hd, _ := h.NewEntry(now, "evenNumbers", &e)
	hash = hd.EntryLink

	Convey("PUT_REQUEST should queue a valid message", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash})
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
	})

	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}
	Convey("GET_REQUEST should return the requested values", t, func() {
		m := h.node.NewMessage(GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", &e))

		m = h.node.NewMessage(GET_REQUEST, GetReq{H: hash, GetMask: GetMaskEntryType})
		r, err = ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(resp.Entry, ShouldBeNil)
		So(resp.EntryType, ShouldEqual, "evenNumbers")

		m = h.node.NewMessage(GET_REQUEST, GetReq{H: hash, GetMask: GetMaskEntry + GetMaskEntryType})
		r, err = ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", &e))
		So(resp.EntryType, ShouldEqual, "evenNumbers")

		m = h.node.NewMessage(GET_REQUEST, GetReq{H: hash, GetMask: GetMaskSources})
		r, err = ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(resp.Entry, ShouldBeNil)
		So(fmt.Sprintf("%v", resp.Sources), ShouldEqual, fmt.Sprintf("[%v]", h.nodeIDStr))
		So(resp.EntryType, ShouldEqual, "")

		m = h.node.NewMessage(GET_REQUEST, GetReq{H: hash, GetMask: GetMaskEntry + GetMaskSources})
		r, err = ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", &e))
		So(fmt.Sprintf("%v", resp.Sources), ShouldEqual, fmt.Sprintf("[%v]", h.nodeIDStr))
		So(resp.EntryType, ShouldEqual, "")
	})

	someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
	e = GobEntry{C: someData}
	_, hd, _ = h.NewEntry(now, "profile", &e)
	profileHash := hd.EntryLink

	le := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String())}
	_, lhd, _ := h.NewEntry(time.Now(), "rating", &le)

	Convey("LINK_REQUEST should store links", t, func() {
		lr := LinkReq{Base: hash, Links: lhd.EntryLink}
		m := h.node.NewMessage(LINK_REQUEST, lr)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")

		// fake the handling of change requests
		err = h.dht.simHandleChangeReqs()
		So(err, ShouldBeNil)

		// check that it got put
		meta, err := h.dht.getLink(hash, "4stars", StatusLive)
		So(err, ShouldBeNil)
		So(meta[0].H, ShouldEqual, hd.EntryLink.String())
	})

	Convey("GETLINK_REQUEST should retrieve link values", t, func() {
		mq := LinkQuery{Base: hash, T: "4stars"}
		m := h.node.NewMessage(GETLINK_REQUEST, mq)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		results := r.(*LinkQueryResp)
		So(results.Links[0].H, ShouldEqual, hd.EntryLink.String())
	})

	Convey("GOSSIP_REQUEST should request and advertise data by idx", t, func() {
		g := GossipReq{MyIdx: 1, YourIdx: 2}
		m := h.node.NewMessage(GOSSIP_REQUEST, g)
		r, err := GossipReceiver(h, m)
		So(err, ShouldBeNil)
		gr := r.(Gossip)
		So(len(gr.Puts), ShouldEqual, 3)
	})

	le2 := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars","LinkAction":"%s"}]}`, hash.String(), profileHash.String(), DelAction)}
	_, lhd2, _ := h.NewEntry(time.Now(), "rating", &le2)

	Convey("LINK_REQUEST with del type should mark a link as deleted", t, func() {
		lr := LinkReq{Base: hash, Links: lhd2.EntryLink}
		m := h.node.NewMessage(LINK_REQUEST, lr)
		r, err := ActionReceiver(h, m)
		So(r, ShouldEqual, "queued")

		// fake the handling of change requests
		err = h.dht.simHandleChangeReqs()
		So(err, ShouldBeNil)

		_, err = h.dht.getLink(hash, "4stars", StatusLive)
		So(err.Error(), ShouldEqual, "No links for 4stars")

		results, err := h.dht.getLink(hash, "4stars", StatusDeleted)
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

	// put a second entry to DHT
	e2 := GobEntry{C: "322"}
	_, hd2, _ := h.NewEntry(now, "evenNumbers", &e2)
	hash2 := hd2.EntryLink
	m2 := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash2})
	ActionReceiver(h, m2)

	Convey("MOD_REQUEST should set hash to modified", t, func() {
		req := ModReq{H: hash, N: hash2}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		results := r.(string)
		So(results, ShouldEqual, "queued")
	})

	Convey("DELETE_REQUEST should set status of hash to deleted", t, func() {
		entry := DelEntry{Hash: hash2, Message: "expired"}
		a := NewDelAction("evenNumbers", entry)
		_, _, entryHash, err := h.doCommit(a, &StatusChange{Action: DelAction, Hash: hash2})

		m := h.node.NewMessage(DEL_REQUEST, DelReq{H: hash2, By: entryHash})
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")

		// fake the handling of change requests
		err = h.dht.simHandleChangeReqs()
		So(err, ShouldBeNil)

		data, entryType, _, status, _ := h.dht.get(hash2, StatusAny, GetMaskAll)
		var e GobEntry
		e.Unmarshal(data)
		So(e.C, ShouldEqual, "322")
		So(entryType, ShouldEqual, "evenNumbers")
		So(status, ShouldEqual, StatusDeleted)
	})
}

func TestDHTDump(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)
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
}

func TestDHT2String(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	Convey("it dump should show the changes count", t, func() {
		So(strings.Index(h.dht.String(), "DHT changes:2") >= 0, ShouldBeTrue)
	})
}

/*
func TestHandleChangeReqs(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestDir(d)

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "{\"prime\":7}"}
	_, hd, err := h.NewEntry(now, "primes", &e)
	if err != nil {
		panic(err)
	}

	m := h.node.NewMessage(PUT_REQUEST, PutReq{H: hd.EntryLink})
	h.dht.puts <- *m

	Convey("handle put request should pull data from source and verify it", t, func() {
		err := h.dht.simHandleChangeReqs()
		So(err, ShouldBeNil)
		data, et,_, _, err := h.dht.get(hd.EntryLink, StatusDefault, GetMaskDefault)
		So(err, ShouldBeNil)
		So(et, ShouldEqual, "primes")
		b, _ := e.Marshal()
		So(fmt.Sprintf("%v", data), ShouldEqual, fmt.Sprintf("%v", b))
	})

}
*/

func (dht *DHT) simHandleChangeReqs() (err error) {
	//	m := <-dht.puts
	//	err = dht.handleChangeReq(&m)
	return
}
