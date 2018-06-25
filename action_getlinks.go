package holochain

import (
	"encoding/json"
	"fmt"
	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	"reflect"
)

//------------------------------------------------------------
// GetLinks

type APIFnGetLinks struct {
	action ActionGetLinks
}

func (fn *APIFnGetLinks) Name() string {
	return fn.action.Name()
}

func (fn *APIFnGetLinks) Args() []Arg {
	return []Arg{{Name: "base", Type: HashArg}, {Name: "tag", Type: StringArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(GetLinksOptions{}), Optional: true}}
}

func (fn *APIFnGetLinks) Call(h *Holochain) (response interface{}, err error) {
	var r interface{}
	a := &fn.action
	r, err = h.dht.Query(a.linkQuery.Base, GETLINK_REQUEST, *a.linkQuery)

	if err == nil {
		switch t := r.(type) {
		case *LinkQueryResp:
			response = t
			if a.options.Load {
				for i := range t.Links {
					var hash Hash
					hash, err = NewHash(t.Links[i].H)
					if err != nil {
						return
					}
					opts := GetOptions{GetMask: GetMaskEntryType + GetMaskEntry, StatusMask: StatusDefault}
					req := GetReq{H: hash, StatusMask: StatusDefault, GetMask: opts.GetMask}
					var rsp interface{}
					rsp, err = callGet(h, req, &opts)
					if err == nil {
						// TODO: bleah, really this should be another of those
						// case statements that choses the encoding baste on
						// entry type, time for a refactor!
						entry := rsp.(GetResp).Entry
						switch content := entry.Content().(type) {
						case string:
							t.Links[i].E = content
						case []byte:
							var j []byte
							j, err = json.Marshal(content)
							if err != nil {
								return
							}
							t.Links[i].E = string(j)
						default:
							err = fmt.Errorf("bad type in entry content: %T:%v", content, content)
						}
						t.Links[i].EntryType = rsp.(GetResp).EntryType
					}
					//TODO better error handling here, i.e break out of the loop and return if error?
				}
			}
		default:
			err = fmt.Errorf("unexpected response type from SendGetLinks: %T", t)
		}
	}
	return
}

type ActionGetLinks struct {
	linkQuery *LinkQuery
	options   *GetLinksOptions
}

func NewGetLinksAction(linkQuery *LinkQuery, options *GetLinksOptions) *ActionGetLinks {
	a := ActionGetLinks{linkQuery: linkQuery, options: options}
	return &a
}

func (a *ActionGetLinks) Name() string {
	return "getLinks"
}

func (a *ActionGetLinks) SysValidation(h *Holochain, d *EntryDef, pkg *Package, sources []peer.ID) (err error) {
	//@TODO what sys level getlinks validation?  That they are all valid hash format for the DNA?
	return
}

func (a *ActionGetLinks) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	lq := msg.Body.(LinkQuery)
	var r LinkQueryResp
	r.Links, err = dht.GetLinks(lq.Base, lq.T, lq.StatusMask)
	response = &r

	return
}
