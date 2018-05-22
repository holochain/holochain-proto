package holochain

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
