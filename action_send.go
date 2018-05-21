package holochain

import (
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"reflect"
	"time"
)

//------------------------------------------------------------
// Send

type Callback struct {
	Function string
	ID       string
	zomeType string
}

type SendOptions struct {
	Callback *Callback
	Timeout  int
}

type ActionSend struct {
	to      peer.ID
	msg     AppMsg
	options *SendOptions
}

type APIFnSend struct {
	action ActionSend
}

func (fn *APIFnSend) Name() string {
	return "send"
}

func (fn *APIFnSend) Args() []Arg {
	return []Arg{{Name: "to", Type: HashArg}, {Name: "msg", Type: MapArg}, {Name: "options", Type: MapArg, MapType: reflect.TypeOf(SendOptions{}), Optional: true}}
}

func (fn *APIFnSend) Call(h *Holochain) (response interface{}, err error) {
	var r interface{}
	var timeout time.Duration
	a := &fn.action
	if a.options != nil {
		timeout = time.Duration(a.options.Timeout) * time.Millisecond
	}
	msg := h.node.NewMessage(APP_MESSAGE, a.msg)
	if a.options != nil && a.options.Callback != nil {
		err = h.SendAsync(ActionProtocol, a.to, msg, a.options.Callback, timeout)
	} else {

		r, err = h.Send(h.node.ctx, ActionProtocol, a.to, msg, timeout)
		if err == nil {
			response = r.(AppMsg).Body
		}
	}
	return
}

func (a *ActionSend) Name() string {
	return "send"
}

func (a *ActionSend) Receive(dht *DHT, msg *Message) (response interface{}, err error) {
	t := msg.Body.(AppMsg)
	var r Ribosome
	r, _, err = dht.h.MakeRibosome(t.ZomeType)
	if err != nil {
		return
	}
	rsp := AppMsg{ZomeType: t.ZomeType}
	rsp.Body, err = r.Receive(peer.IDB58Encode(msg.From), t.Body)
	if err == nil {
		response = rsp
	}
	return
}
