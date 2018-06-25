package holochain

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/HC-Interns/holochain-proto/hash"
	b58 "github.com/jbenet/go-base58"
	peer "github.com/libp2p/go-libp2p-peer"
	. "github.com/smartystreets/goconvey/convey"
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
		So(h.dht.Exists(ID, StatusLive), ShouldBeNil)
		_, et, _, status, err := h.dht.Get(h.dnaHash, StatusLive, GetMaskAll)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)
		So(et, ShouldEqual, DNAEntryType)

	})

	Convey("it should push the agent entry to the DHT at genesis time", t, func() {
		data, et, _, status, err := h.dht.Get(h.agentHash, StatusLive, GetMaskAll)
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
		data, et, _, status, err := h.dht.Get(keyHash, StatusLive, GetMaskAll)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)
		So(et, ShouldEqual, KeyEntryType)
		pubKey, err := h.agent.EncodePubKey()
		So(string(data), ShouldEqual, pubKey)

		data, et, _, status, err = h.dht.Get(keyHash, StatusDefault, GetMaskDefault)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, StatusLive)

		So(string(data), ShouldEqual, pubKey)
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
		msg := h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: hash})
		r, err := h.dht.send(nil, h.node.HashAddr, msg)
		So(err, ShouldBeNil)
		So(r.(HoldResp).Code, ShouldEqual, ReceiptOK)
		hd, _ := h.chain.GetEntryHeader(hash)
		So(hd.EntryLink.Equal(hash), ShouldBeTrue)
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
		a, _ := ae.ToJSON()
		So(resp.Entry.Content().(string), ShouldEqual, a)

		msg = h.node.NewMessage(GET_REQUEST, GetReq{H: HashFromPeerID(h.nodeID), StatusMask: StatusLive})
		r, err = h.dht.send(nil, h.nodeID, msg)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(resp.Entry.Content().(string), ShouldEqual, ae.PublicKey)

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
	msg := h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: hash})
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
		err := h.dht.Change(hash, PUT_REQUEST, HoldReq{EntryHash: hash})
		So(err, ShouldBeNil)

		processChangeRequestsInTesting(h)
		rtp = h.node.routingTable.NearestPeers(hash, AlphaValue)
		// routing table should be updated
		So(fmt.Sprintf("%v", rtp), ShouldEqual, "[<peer.ID S4BFeT> <peer.ID W4HeEG> <peer.ID UfY4We>]")
		// and get from node should get the value
		msg := h.node.NewMessage(GET_REQUEST, GetReq{H: hash, StatusMask: StatusLive})
		r, err := h.dht.send(nil, mt.nodes[3].nodeID, msg)
		So(err, ShouldBeNil)
		resp := r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", e))

		if h.Config.EnableWorldModel {
			// and the world model should show that it's being held
			holding, err := h.world.IsHolding(mt.nodes[3].nodeID, hash)
			So(err, ShouldBeNil)
			So(holding, ShouldBeTrue)
		}
	})
}

func TestActionReceiver(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	Convey("PUT_REQUEST should fail if body isn't a hash", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, "foo")
		_, err := ActionReceiver(h, m)
		So(err.Error(), ShouldEqual, "Unexpected request body type 'string' in put request, expecting holochain.HoldReq")
	})

	Convey("LINK_REQUEST should fail if body not a good linking request", t, func() {
		m := h.node.NewMessage(LINK_REQUEST, "foo")
		_, err := ActionReceiver(h, m)
		So(err.Error(), ShouldEqual, "Unexpected request body type 'string' in link request, expecting holochain.HoldReq")
	})

	now := time.Unix(1, 1) // pick a constant time so the test will always work
	e := GobEntry{C: "124"}
	_, hd, _ := h.NewEntry(now, "evenNumbers", &e)
	hash := hd.EntryLink

	Convey("PUT_REQUEST should queue a valid message", t, func() {
		m := h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: hash})
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r.(HoldResp).Code, ShouldEqual, ReceiptOK)
		data, _ := MakeReceiptData(m, ReceiptOK)
		matches, err := h.VerifySignature(r.(HoldResp).Signature, string(data), h.agent.PubKey())
		So(err, ShouldBeNil)
		So(matches, ShouldBeTrue)
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
		So(resp.Entry.C, ShouldEqual, nil)
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
		So(resp.Entry.C, ShouldEqual, nil)
		So(fmt.Sprintf("%v", resp.Sources), ShouldEqual, fmt.Sprintf("[%v]", h.nodeIDStr))
		So(resp.EntryType, ShouldEqual, "evenNumbers") // you allways get the entry type even if not in getmask at this level (ActionReceiver) because we have to be able to look up the definition to interpret the contents

		m = h.node.NewMessage(GET_REQUEST, GetReq{H: hash, GetMask: GetMaskEntry + GetMaskSources})
		r, err = ActionReceiver(h, m)
		So(err, ShouldBeNil)
		resp = r.(GetResp)
		So(fmt.Sprintf("%v", resp.Entry), ShouldEqual, fmt.Sprintf("%v", e))
		So(fmt.Sprintf("%v", resp.Sources), ShouldEqual, fmt.Sprintf("[%v]", h.nodeIDStr))
		So(resp.EntryType, ShouldEqual, "evenNumbers") // you allways get the entry type even if not in getmask at this level (ActionReceiver) because we have to be able to look up the definition to interpret the contents
	})

	someData := `{"firstName":"Zippy","lastName":"Pinhead"}`
	e = GobEntry{C: someData}
	_, hd, _ = h.NewEntry(now, "profile", &e)
	profileHash := hd.EntryLink

	le := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"},{"Base":"%s","Link":"%s","Tag":"3stars"}]}`, hash.String(), profileHash.String(), hash.String(), h.agentHash)}
	_, lhd, _ := h.NewEntry(time.Now(), "rating", &le)

	Convey("LINK_REQUEST should store links", t, func() {
		lr := HoldReq{RelatedHash: hash, EntryHash: lhd.EntryLink}
		m := h.node.NewMessage(LINK_REQUEST, lr)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r.(HoldResp).Code, ShouldEqual, ReceiptOK)
		data, _ := MakeReceiptData(m, ReceiptOK)
		matches, err := h.VerifySignature(r.(HoldResp).Signature, string(data), h.agent.PubKey())
		So(err, ShouldBeNil)
		So(matches, ShouldBeTrue)

		// check that it got put
		meta, err := h.dht.GetLinks(hash, "4stars", StatusLive)
		So(err, ShouldBeNil)
		So(meta[0].H, ShouldEqual, hd.EntryLink.String())
	})

	e2 := GobEntry{C: "322"}
	hash2, _ := e2.Sum(h.hashSpec)

	e3 := GobEntry{C: "324"}
	hash3, _ := e3.Sum(h.hashSpec)

	Convey("LINK_REQUEST of unknown hash should get queued for retry", t, func() {
		lr := HoldReq{RelatedHash: hash2, EntryHash: hash3}
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

	le2 := GobEntry{C: fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars","LinkAction":"%s"}]}`, hash.String(), profileHash.String(), DelLinkAction)}
	_, lhd2, _ := h.NewEntry(time.Now(), "rating", &le2)

	Convey("LINK_REQUEST with del type should mark a link as deleted", t, func() {
		lr := HoldReq{RelatedHash: hash, EntryHash: lhd2.EntryLink}
		m := h.node.NewMessage(LINK_REQUEST, lr)
		r, err := ActionReceiver(h, m)
		So(r.(HoldResp).Code, ShouldEqual, ReceiptOK)
		data, _ := MakeReceiptData(m, ReceiptOK)
		matches, err := h.VerifySignature(r.(HoldResp).Signature, string(data), h.agent.PubKey())
		So(err, ShouldBeNil)
		So(matches, ShouldBeTrue)

		results, err := h.dht.GetLinks(hash, "4stars", StatusLive)
		So(err, ShouldBeNil)
		So(len(results), ShouldEqual, 0)

		results, err = h.dht.GetLinks(hash, "4stars", StatusDeleted)
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
		req := HoldReq{RelatedHash: hash2, EntryHash: hash3}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeUnknownHashQueuedForRetry)
		So(len(h.dht.retryQueue), ShouldEqual, 1)
		<-h.dht.retryQueue // unload the queue
	})

	// put a second entry to DHT
	h.NewEntry(now, "evenNumbers", &e2)
	m2 := h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: hash2})
	ActionReceiver(h, m2)

	Convey("MOD_REQUEST should set hash to modified", t, func() {
		req := HoldReq{RelatedHash: hash, EntryHash: hash2}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r.(HoldResp).Code, ShouldEqual, ReceiptOK)
		data, _ := MakeReceiptData(m, ReceiptOK)
		matches, err := h.VerifySignature(r.(HoldResp).Signature, string(data), h.agent.PubKey())
		So(err, ShouldBeNil)
		So(matches, ShouldBeTrue)
	})

	Convey("DELETE_REQUEST should set status of hash to deleted", t, func() {
		entry := DelEntry{Hash: hash2, Message: "expired"}
		a := NewDelAction(entry)
		_, err := h.doCommit(a, NullHash())
		entryHash := a.header.EntryLink
		m := h.node.NewMessage(DEL_REQUEST, HoldReq{RelatedHash: hash2, EntryHash: entryHash})
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r.(HoldResp).Code, ShouldEqual, ReceiptOK)
		data, _ := MakeReceiptData(m, ReceiptOK)
		matches, err := h.VerifySignature(r.(HoldResp).Signature, string(data), h.agent.PubKey())
		So(err, ShouldBeNil)
		So(matches, ShouldBeTrue)

		data, entryType, _, status, _ := h.dht.Get(hash2, StatusAny, GetMaskAll)
		var e GobEntry
		e.Unmarshal(data)
		So(e.C, ShouldEqual, "322")
		So(entryType, ShouldEqual, "evenNumbers")
		So(status, ShouldEqual, StatusDeleted)
	})

	Convey("DELETE_REQUEST of unknown hash should get queued for retry", t, func() {
		req := HoldReq{RelatedHash: hash3, EntryHash: hash3}
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
		                           	So(r.(HoldResp).Code, ShouldEqual, ReceiptOK)

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

	ht := h.dht.ht.(*BuntHT)
	Convey("dht dump of index 1 should show the agent put", t, func() {
		msg, _ := ht.GetIdxMessage(1)
		f, _ := msg.Fingerprint()
		msgStr := msg.String()

		str, err := ht.dumpIdx(1)
		So(err, ShouldBeNil)

		So(strings.Index(str, fmt.Sprintf("MSG (fingerprint %v)", f)) >= 0, ShouldBeTrue)
		So(strings.Index(str, msgStr) >= 0, ShouldBeTrue)
	})

	Convey("dht dump of index 99 should return err", t, func() {
		_, err := ht.dumpIdx(99)
		So(err.Error(), ShouldEqual, "no such change index")
	})

	reviewHash := commit(h, "review", "this is my bogus review of the user")
	commit(h, "rating", fmt.Sprintf(`{"Links":[{"Base":"%s","Link":"%s","Tag":"4stars"}]}`, h.nodeIDStr, reviewHash.String()))

	Convey("dht.String() should produce human readable DHT", t, func() {
		dump := h.dht.String()
		So(dump, ShouldContainSubstring, "DHT changes: 5")
		d, _ := ht.dumpIdx(1)
		So(dump, ShouldContainSubstring, d)
		d, _ = ht.dumpIdx(2)
		So(dump, ShouldContainSubstring, d)

		So(dump, ShouldContainSubstring, "DHT entries:")
		So(dump, ShouldContainSubstring, fmt.Sprintf("Hash--%s (status 1)", h.nodeIDStr))
		pk, _ := h.agent.PubKey().Bytes()
		So(dump, ShouldContainSubstring, fmt.Sprintf("Value: %s", string(b58.Encode(pk))))
		So(dump, ShouldContainSubstring, fmt.Sprintf("Sources: %s", h.nodeIDStr))

		So(dump, ShouldContainSubstring, fmt.Sprintf("Linked to: %s with tag %s", reviewHash, "4stars"))
	})

	Convey("dht.JSON() should output DHT formatted as JSON string", t, func() {
		dump, err := h.dht.JSON()
		So(err, ShouldBeNil)
		d, _ := ht.dumpIdxJSON(1)
		So(NormaliseJSON(dump), ShouldContainSubstring, NormaliseJSON(d))
		d, _ = ht.dumpIdxJSON(2)
		So(NormaliseJSON(dump), ShouldContainSubstring, NormaliseJSON(d))

		json := NormaliseJSON(dump)
		matched, err := regexp.MatchString(`{"dht_changes":\[.*\],"dht_entries":\[.*\]}`, json)
		So(err, ShouldBeNil)
		So(matched, ShouldBeTrue)
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
		req := HoldReq{RelatedHash: hash, EntryHash: hash2}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeUnknownHashQueuedForRetry)

		// pause for a few retires
		h.node.stoppers[RetryingStopper] = h.TaskTicker(time.Millisecond*10, RetryTask)
		time.Sleep(time.Millisecond * 25)

		// add the entries and get them into the DHT
		h.NewEntry(time.Now(), "profile", &e)
		h.NewEntry(time.Now(), "profile", &e2)
		m = h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: hash})
		err = h.dht.Put(m, "profile", hash, h.nodeID, []byte(d1), StatusLive)
		So(err, ShouldBeNil)
		m = h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: hash2})
		err = h.dht.Put(m, "profile", hash2, h.nodeID, []byte(d2), StatusLive)
		So(err, ShouldBeNil)

		_, _, _, status, _ := h.dht.Get(hash, StatusAny, GetMaskAll)
		So(status, ShouldEqual, StatusLive)

		// wait for next retry
		time.Sleep(time.Millisecond * 40)

		_, _, _, status, _ = h.dht.Get(hash, StatusAny, GetMaskAll)
		So(status, ShouldEqual, StatusModified)

		// stop retrying for next test
		stop := h.node.stoppers[RetryingStopper]
		h.node.stoppers[RetryingStopper] = nil
		stop <- true

	})

	Convey("retries should be limited", t, func() {
		e3 := GobEntry{C: `{"firstName":"Zappy","lastName":"Pinhead"}`}
		hash3, _ := e3.Sum(h.hashSpec)
		e4 := GobEntry{C: `{"firstName":"Zuppy","lastName":"Pinhead"}`}
		hash4, _ := e4.Sum(h.hashSpec)
		req := HoldReq{RelatedHash: hash3, EntryHash: hash4}
		m := h.node.NewMessage(MOD_REQUEST, req)
		r, err := ActionReceiver(h, m)
		So(err, ShouldBeNil)
		So(r, ShouldEqual, DHTChangeUnknownHashQueuedForRetry)
		So(len(h.dht.retryQueue), ShouldEqual, 1)

		interval := time.Millisecond * 10
		h.node.stoppers[RetryingStopper] = h.TaskTicker(interval, RetryTask)
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
				response, err := callGet(h2, req, &options)
				if err != nil {
					fmt.Printf("FAIL   : %v couldn't get from %v err: %err\n", h2.nodeID, h1.nodeID, err)
				} else {
					e := response.(GetResp).Entry
					responseStr := fmt.Sprintf("%v", e.Content())
					pk, _ := h1.agent.EncodePubKey()
					expectedResponseStr := pk
					if responseStr == expectedResponseStr {
						connections += 1
						//fmt.Printf("SUCCESS: %v got from          %v\n", h2.nodeID, h1.nodeID)
					} else {
						//fmt.Printf("Expected:%s\n", expectedResponseStr)
						//fmt.Printf("Got     :%s\n", responseStr)
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
				fn := &APIFnGetLinks{action: *NewGetLinksAction(
					&LinkQuery{
						Base:       HashFromPeerID(h1.nodeID),
						T:          "statement",
						StatusMask: options.StatusMask,
					}, &options)}
				response, err := fn.Call(h2)
				So(err, ShouldBeNil)
				So(fmt.Sprintf("%v", response), ShouldEqual, fmt.Sprintf("&{[{%v this statement by node %d (%v) review  %s}]}", hashes[i], i, h1.nodeID, h1.nodeID.Pretty()))
			}
		}
	})
}

func TestDHTMakeReciept(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")

	Convey("it should make a receipt and signature", t, func() {
		msg := h.node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: hash})

		data, err := MakeReceiptData(msg, ReceiptOK)
		So(err, ShouldBeNil)
		sig, err := h.Sign(data)
		if err != nil {
			panic(err)
		}

		receiptSig, err := h.dht.MakeReceiptSignature(msg, ReceiptOK)
		So(err, ShouldBeNil)
		So(receiptSig.Equal(sig), ShouldBeTrue)

		matches, err := h.VerifySignature(receiptSig, string(data), h.agent.PubKey())
		So(err, ShouldBeNil)
		So(matches, ShouldBeTrue)

		holdResp, err := h.dht.MakeHoldResp(msg, StatusRejected)
		So(err, ShouldBeNil)
		So(holdResp.Code, ShouldEqual, ReceiptRejected)
		matches, err = h.VerifySignature(holdResp.Signature, string(data), h.agent.PubKey())
		So(err, ShouldBeNil)
		So(matches, ShouldBeFalse)

		holdResp, err = h.dht.MakeHoldResp(msg, StatusLive)
		So(err, ShouldBeNil)
		So(holdResp.Code, ShouldEqual, ReceiptOK)
		matches, err = h.VerifySignature(holdResp.Signature, string(data), h.agent.PubKey())
		So(err, ShouldBeNil)
		So(matches, ShouldBeTrue)
	})
}

func processChangeRequestsInTesting(h *Holochain) {
	for len(h.dht.changeQueue) > 0 {
		req := <-h.dht.changeQueue
		err := handleChangeRequests(h.dht, req)
		if err != nil {
			panic(err)
		}
	}
}
