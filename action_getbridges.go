package holochain

//------------------------------------------------------------
// GetBridges

type APIFnGetBridges struct {
}

func (a *APIFnGetBridges) Name() string {
	return "getBridges"
}

func (a *APIFnGetBridges) Args() []Arg {
	return []Arg{}
}

func (a *APIFnGetBridges) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.GetBridges()
	return
}
