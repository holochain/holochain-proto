package holochain

import (
	"errors"
	"fmt"
	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	"reflect"
)

//------------------------------------------------------------
// Get

type APIFnGet struct {
	action ActionGet
}

func (fn *APIFnGet) Name() string {
	return fn.action.Name()
}

func (fn *APIFnGet) Args() []Arg {
	return []Arg{{Name: "hash", Type: HashArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(GetOptions{}), Optional: true}}
}

func callGet(h *Holochain, req GetReq, options *GetOptions) (response interface{}, err error) {
	a := ActionGet{req: req, options: options}
	fn := &APIFnGet{action: a}
	response, err = fn.Call(h)
	return
}

func (fn *APIFnGet) Call(h *Holochain) (response interface{}, err error) {
	a := &fn.action
	if a.options.Local {
		response, err = a.getLocal(h.chain)
		return
	}
	if a.options.Bundle {
		bundle := h.Chain().BundleStarted()
		if bundle == nil {
			err = ErrBundleNotStarted
			return
		}
		response, err = a.getLocal(bundle.chain)
		return
	}
	rsp, err := h.dht.Query(a.req.H, GET_REQUEST, a.req)
	if err != nil {

		// follow the modified hash
		if a.req.StatusMask == StatusDefault && err == ErrHashModified {
			var hash Hash
			hash, err = NewHash(rsp.(GetResp).FollowHash)
			if err != nil {
				return
			}
			if hash.String() == a.req.H.String() {
				err = errors.New("FollowHash loop detected")
				return
			}
			req := GetReq{H: hash, StatusMask: StatusDefault, GetMask: a.options.GetMask}
			modResp, err := callGet(h, req, a.options)
			if err == nil {
				response = modResp
			}
		}
		return
	}
	switch t := rsp.(type) {
	case GetResp:
		response = t
	default:
		err = fmt.Errorf("expected GetResp response from GET_REQUEST, got: %T", t)
		return
	}
	return
}

type ActionGet struct {
	req     GetReq
	options *GetOptions
}

func (a *ActionGet) Name() string {
	return "get"
}

func (a *ActionGet) getLocal(chain *Chain) (resp GetResp, err error) {
	var entry Entry
	var entryType string
	entry, entryType, err = chain.GetEntry(a.req.H)
	if err != nil {
		return
	}
	resp = GetResp{Entry: *entry.(*GobEntry)}
	mask := a.options.GetMask
	resp.EntryType = entryType
	if (mask & GetMaskEntry) != 0 {
		resp.Entry = *entry.(*GobEntry)
		resp.EntryType = entryType
	}
	return
}

func (a *ActionGet) SysValidation(h *Holochain, def *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	return
}

func (a *ActionGet) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	var entryData []byte
	//var status int
	req := msg.Body.(GetReq)
	mask := req.GetMask
	if mask == GetMaskDefault {
		mask = GetMaskEntry
	}
	resp := GetResp{}
	// always get the entry type despite what the mas says because we need it for the switch below.
	entryData, resp.EntryType, resp.Sources, _, err = dht.Get(req.H, req.StatusMask, req.GetMask|GetMaskEntryType)
	if err == nil {
		if (mask & GetMaskEntry) != 0 {
			switch resp.EntryType {
			case DNAEntryType:
				// TODO: make this add the requester to the blockedlist rather than panicing, see ticket #421
				err = errors.New("nobody should actually get the DNA!")
				return
			case KeyEntryType:
				resp.Entry = GobEntry{C: string(entryData)}
			default:
				var e GobEntry
				err = e.Unmarshal(entryData)
				if err != nil {
					return
				}
				resp.Entry = e
			}
		}
	} else {
		if err == ErrHashModified {
			resp.FollowHash = string(entryData)
		} else if err == ErrHashNotFound {
			closest := dht.h.node.betterPeersForHash(&req.H, msg.From, true, CloserPeerCount)
			if len(closest) > 0 {
				err = nil
				resp := CloserPeersResp{}
				resp.CloserPeers = dht.h.node.peers2PeerInfos(closest)
				response = resp
				return
			}
		}
	}

	response = resp
	return
}
