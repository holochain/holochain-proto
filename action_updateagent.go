package holochain

import (
	"errors"
	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	"reflect"
)

//------------------------------------------------------------
// ModAgent

type APIFnModAgent struct {
	Identity   AgentIdentity
	Revocation string
}

func (fn *APIFnModAgent) Args() []Arg {
	return []Arg{{Name: "options", Type: MapArg, MapType: reflect.TypeOf(ModAgentOptions{})}}
}

func (fn *APIFnModAgent) Name() string {
	return "updateAgent"
}
func (fn *APIFnModAgent) Call(h *Holochain) (response interface{}, err error) {
	var ok bool
	var newAgent LibP2PAgent = *h.agent.(*LibP2PAgent)
	if fn.Identity != "" {
		newAgent.identity = fn.Identity
		ok = true
	}

	var revocation *SelfRevocation
	if fn.Revocation != "" {
		err = newAgent.GenKeys(nil)
		if err != nil {
			return
		}
		revocation, err = NewSelfRevocation(h.agent.PrivKey(), newAgent.PrivKey(), []byte(fn.Revocation))
		if err != nil {
			return
		}
		ok = true
	}
	if !ok {
		err = errors.New("expecting identity and/or revocation option")
	} else {

		//TODO: synchronize this, what happens if two new agent request come in back to back?
		h.agent = &newAgent
		// add a new agent entry and update
		var agentHash Hash
		_, agentHash, err = h.AddAgentEntry(revocation)
		if err != nil {
			return
		}
		h.agentTopHash = agentHash

		// if there was a revocation put the new key to the DHT and then reset the node ID data
		// TODO make sure this doesn't introduce race conditions in the DHT between new and old identity #284
		if revocation != nil {
			err = h.dht.putKey(&newAgent)
			if err != nil {
				return
			}

			// send the modification request for the old key
			var oldKey, newKey Hash
			oldPeer := h.nodeID
			oldKey, err = NewHash(h.nodeIDStr)
			if err != nil {
				panic(err)
			}

			h.nodeID, h.nodeIDStr, err = h.agent.NodeID()
			if err != nil {
				return
			}

			newKey, err = NewHash(h.nodeIDStr)
			if err != nil {
				panic(err)
			}

			// close the old node and add the new node
			// TODO currently ignoring the error from node.Close() is this OK?
			h.node.Close()
			h.createNode()

			h.dht.Change(oldKey, MOD_REQUEST, HoldReq{RelatedHash: oldKey, EntryHash: newKey})

			warrant, _ := NewSelfRevocationWarrant(revocation)
			var data []byte
			data, err = warrant.Encode()
			if err != nil {
				return
			}

			// TODO, this isn't really a DHT send, but a management send, so the key is bogus.  have to work this out...
			h.dht.Change(oldKey, LISTADD_REQUEST,
				ListAddReq{
					ListType:    BlockedList,
					Peers:       []string{peer.IDB58Encode(oldPeer)},
					WarrantType: SelfRevocationType,
					Warrant:     data,
				})

		}

		response = agentHash
	}
	return
}
