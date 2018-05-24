package holochain

import (
	"fmt"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
)

//------------------------------------------------------------
// ListAdd

type ActionListAdd struct {
	list PeerList
}

func NewListAddAction(peerList PeerList) *ActionListAdd {
	a := ActionListAdd{list: peerList}
	return &a
}

func (a *ActionListAdd) Name() string {
	return "put"
}

var prefix string = "List add request rejected on warrant failure"

func (a *ActionListAdd) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(ListAddReq)
	a.list.Type = PeerListType(t.ListType)
	a.list.Records = make([]PeerRecord, 0)
	var pid peer.ID
	for _, pStr := range t.Peers {
		pid, err = peer.IDB58Decode(pStr)
		if err != nil {
			return
		}
		r := PeerRecord{ID: pid}
		a.list.Records = append(a.list.Records, r)
	}

	// validate the warrant sent with the list add request
	var w Warrant
	w, err = DecodeWarrant(t.WarrantType, t.Warrant)
	if err != nil {
		err = fmt.Errorf("%s: unable to decode warrant (%v)", prefix, err)
		return
	}

	err = w.Verify(dht.h)
	if err != nil {
		err = fmt.Errorf("%s: %v", prefix, err)
		return
	}

	// TODO verify that the warrant, if valid, is sufficient to allow list addition #300

	err = dht.addToList(msg, a.list)
	if err != nil {
		return
	}

	// special case to add blockedlist peers to node cache and delete them from the gossipers list
	if a.list.Type == BlockedList {
		for _, node := range a.list.Records {
			dht.h.node.Block(node.ID)
			dht.DeleteGossiper(node.ID) // ignore error
		}
	}
	response = DHTChangeOK
	return
}
