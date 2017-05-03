package holochain

import (
	"fmt"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
	"time"
)

func TestNewDHT(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
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
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	err := h.dht.SetupDHT()
	Convey("it should add the holochain ID to the DHT", t, func() {
		So(err, ShouldBeNil)
		ID := h.DNAHash()
		So(h.dht.exists(ID, StatusLive), ShouldBeNil)
		_, et, status, err := h.dht.get(h.dnaHash, StatusLive)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)
		So(et, ShouldEqual, DNAEntryType)

	})

	Convey("it should push the agent entry to the DHT at genesis time", t, func() {
		data, et, status, err := h.dht.get(h.agentHash, StatusLive)
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
		keyHash, _ := NewHash(peer.IDB58Encode(h.id))
		data, et, status, err := h.dht.get(keyHash, StatusLive)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)
		So(et, ShouldEqual, KeyEntryType)
		So(string(data), ShouldEqual, string([]byte(h.id)))

	})
}

func TestPutGetModDel(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	dht := h.dht
	var id = h.id
	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	var idx int
	Convey("It should store and retrieve", t, func() {
		err := dht.put(nil, "someType", hash, id, []byte("some value"), StatusLive)
		So(err, ShouldBeNil)
		idx, _ = dht.GetIdx()

		data, entryType, status, err := dht.get(hash, StatusLive)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, StatusLive)

		badhash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
		data, entryType, _, err = dht.get(badhash, StatusLive)
		So(entryType, ShouldEqual, "")
		So(err, ShouldEqual, ErrHashNotFound)
	})

	Convey("mod should move the hash to the modified status and record replacedBy link", t, func() {
		m := h.node.NewMessage(MOD_REQUEST, hash)

		newhashStr := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh4"
		newhash, _ := NewHash(newhashStr)

		err := dht.mod(m, hash, newhash)
		So(err, ShouldBeNil)
		data, entryType, status, err := dht.get(hash, StatusAny)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, StatusModified)

		afterIdx, _ := dht.GetIdx()

		So(afterIdx-idx, ShouldEqual, 1)

		data, entryType, status, err = dht.get(hash, StatusLive)
		So(err, ShouldEqual, ErrHashNotFound)

		data, entryType, status, err = dht.get(hash, StatusDefault)
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

		data, entryType, status, err := dht.get(hash, StatusAny)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, StatusDeleted)

		afterIdx, _ := dht.GetIdx()

		So(afterIdx-idx, ShouldEqual, 2)

		data, entryType, status, err = dht.get(hash, StatusLive)
		So(err, ShouldEqual, ErrHashNotFound)

		data, entryType, status, err = dht.get(hash, StatusDefault)
		So(err, ShouldEqual, ErrHashDeleted)

	})

}

func TestLinking(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)
	var h Holochain
	h.rootPath = d
	os.MkdirAll(h.DBPath(), os.ModePerm)
	dht := NewDHT(&h)
	baseStr := "QmZcUPvPhD1Xvk6mwijYF8AfR3mG31S1YsEfHG4khrFPRr"
	base, err := NewHash(baseStr)
	if err != nil {
		panic(err)
	}
	linkHash1Str := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh1"
	// linkHash1, _ := NewHash(linkHash1Str)
	linkHash2Str := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2"
	//	linkHash2, _ := NewHash(linkHash2Str)
	Convey("It should fail if hash doesn't exist", t, func() {
		err := dht.putLink(nil, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldEqual, ErrHashNotFound)

		v, err := dht.getLink(base, "tag foo", StatusLive)
		So(v, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	var id peer.ID
	err = dht.put(nil, "someType", base, id, []byte("some value"), StatusLive)
	if err != nil {
		panic(err)
	}

	Convey("It should store and retrieve links values on a base", t, func() {
		data, err := dht.getLink(base, "tag foo", StatusLive)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "No links for tag foo")

		err = dht.putLink(nil, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)

		err = dht.putLink(nil, baseStr, linkHash2Str, "tag foo")
		So(err, ShouldBeNil)

		err = dht.putLink(nil, baseStr, linkHash1Str, "tag bar")
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

		err := dht.delLink(nil, badHashStr, linkHash1Str, "tag foo")
		So(err, ShouldEqual, ErrHashNotFound)
		err = dht.delLink(nil, baseStr, badHashStr, "tag foo")
		So(err, ShouldEqual, ErrLinkNotFound)
		err = dht.delLink(nil, baseStr, linkHash1Str, "tag baz")
		So(err, ShouldEqual, ErrLinkNotFound)
	})

	Convey("It should delete links", t, func() {
		err := dht.delLink(nil, baseStr, linkHash1Str, "tag bar")
		So(err, ShouldBeNil)
		data, err := dht.getLink(base, "tag bar", StatusLive)
		So(err.Error(), ShouldEqual, "No links for tag bar")

		err = dht.delLink(nil, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)
		data, err = dht.getLink(base, "tag foo", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)

		err = dht.delLink(nil, baseStr, linkHash2Str, "tag foo")
		So(err, ShouldBeNil)
		data, err = dht.getLink(base, "tag foo", StatusLive)
		So(err.Error(), ShouldEqual, "No links for tag foo")
	})
}

func TestFindNodeForHash(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("It should find a node", t, func() {

		// for now the node it finds is ourself for any hash because we haven't implemented
		// anything about neighborhoods or other nodes...
		hash, err := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		if err != nil {
			panic(err)
		}
		node, err := h.dht.FindNodeForHash(hash)
		So(err, ShouldBeNil)
		So(node.HashAddr.Pretty(), ShouldEqual, h.id.Pretty())
	})
}

func TestSend(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	node, err := NewNode("/ip4/127.0.0.1/tcp/1234", h.id, h.Agent().PrivKey())
	if err != nil {
		panic(err)
	}
	defer node.Close()

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("send GET_REQUEST message for non existent hash should get error", t, func() {
		r, err := h.dht.send(node.HashAddr, GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		So(r, ShouldBeNil)
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
		So(fmt.Sprintf("%v", r), ShouldEqual, fmt.Sprintf("%v", &e))
	})
}

func TestDHTReceiver(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

	Convey("PUT_REQUEST should fail if body isn't a hash", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, "foo")
		_, err := DHTReceiver(h, m)
		So(err.Error(), ShouldEqual, "Unexpected request body type 'string' in put request")
	})

	Convey("LINK_REQUEST should fail if body not a good linking request", t, func() {
		m := h.node.NewMessage(LINK_REQUEST, "foo")
		_, err := DHTReceiver(h, m)
		So(err.Error(), ShouldEqual, "Unexpected request body type 'string' in link request")
	})

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("LINK_REQUEST should fail if hash doesn't exist", t, func() {
		me := LinkReq{Base: hash, Links: hash}
		m := h.node.NewMessage(LINK_REQUEST, me)
		_, err := DHTReceiver(h, m)
		So(err.Error(), ShouldEqual, "hash not found")
	})

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "124"}
	_, hd, _ := h.NewEntry(now, "evenNumbers", &e)
	hash = hd.EntryLink

	Convey("PUT_REQUEST should queue a valid message", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash})
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")
	})

	if err := h.dht.simHandleChangeReqs(); err != nil {
		panic(err)
	}
	Convey("GET_REQUEST should return the value of the has", t, func() {
		m := h.node.NewMessage(GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", r), ShouldEqual, fmt.Sprintf("%v", &e))
	})

	someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
	e = GobEntry{C: someData}
	_, hd, _ = h.NewEntry(now, "profile", &e)
	profileHash := hd.EntryLink

	ee := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, hash.String(), profileHash.String())}
	_, le, _ := h.NewEntry(time.Now(), "rating", &ee)

	Convey("LINK_REQUEST should store links", t, func() {
		lr := LinkReq{Base: hash, Links: le.EntryLink}
		m := h.node.NewMessage(LINK_REQUEST, lr)
		r, err := DHTReceiver(h, m)
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
		r, err := DHTReceiver(h, m)
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
		So(len(gr.Puts), ShouldEqual, 4)
	})

	Convey("DELETELINK_REQUEST should mark a link as deleted", t, func() {
		mq := DelLinkReq{Base: hash, Link: profileHash, Tag: "4stars"}
		m := h.node.NewMessage(DELETELINK_REQUEST, mq)
		_, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		//		So(r, ShouldEqual, "queued")

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
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		results := r.(*LinkQueryResp)
		So(results.Links[0].H, ShouldEqual, hd.EntryLink.String())
	})

	// put a second entry to DHT
	e2 := GobEntry{C: "321"}
	_, hd2, _ := h.NewEntry(now, "oddNumbers", &e2)
	hash2 := hd2.EntryLink
	m2 := h.node.NewMessage(PUT_REQUEST, PutReq{H: hash2})
	DHTReceiver(h, m2)

	Convey("MOD_REQUEST should set hash to modified", t, func() {
		req := ModReq{H: hash, N: hash2}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		results := r.(string)
		So(results, ShouldEqual, "queued")
	})

	Convey("DELETE_REQUEST should set status of hash to deleted", t, func() {
		m := h.node.NewMessage(DEL_REQUEST, DelReq{H: hash2})
		r, err := DHTReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, "queued")

		// fake the handling of change requests
		err = h.dht.simHandleChangeReqs()
		So(err, ShouldBeNil)

		data, entryType, status, _ := h.dht.get(hash2, StatusAny)
		var e GobEntry
		e.Unmarshal(data)
		So(e.C, ShouldEqual, "321")
		So(entryType, ShouldEqual, "oddNumbers")
		So(status, ShouldEqual, StatusDeleted)
	})
}

/*
func TestHandleChangeReqs(t *testing.T) {
	d, _, h := prepareTestChain("test")
	defer cleanupTestDir(d)

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
		data, et, _, err := h.dht.get(hd.EntryLink)
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
