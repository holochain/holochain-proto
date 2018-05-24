package holochain

import (
	ic "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

//------------------------------------------------------------
// Sign

type APIFnSign struct {
	data []byte
}

func (a *APIFnSign) Name() string {
	return "sign"
}

func (a *APIFnSign) Args() []Arg {
	return []Arg{{Name: "data", Type: StringArg}}
}

func (a *APIFnSign) Call(h *Holochain) (response interface{}, err error) {
	var sig Signature
	sig, err = h.Sign(a.data)
	if err != nil {
		return
	}
	response = sig.B58String()
	return
}

//------------------------------------------------------------
// VerifySignature

type APIFnVerifySignature struct {
	b58signature string
	data         string
	b58pubKey    string
}

func (a *APIFnVerifySignature) Name() string {
	return "verifySignature"
}

func (a *APIFnVerifySignature) Args() []Arg {
	return []Arg{{Name: "signature", Type: StringArg}, {Name: "data", Type: StringArg}, {Name: "pubKey", Type: StringArg}}
}

func (a *APIFnVerifySignature) Call(h *Holochain) (response interface{}, err error) {
	var b bool
	var pubKey ic.PubKey
	sig := SignatureFromB58String(a.b58signature)

	pubKey, err = DecodePubKey(a.b58pubKey)

	b, err = h.VerifySignature(sig, a.data, pubKey)
	if err != nil {
		return
	}
	response = b
	return
}
