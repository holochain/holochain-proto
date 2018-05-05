package holochain

//------------------------------------------------------------
// Property

type APIFnProperty struct {
	prop string
}

func (a *APIFnProperty) Name() string {
	return "property"
}

func (a *APIFnProperty) Args() []Arg {
	return []Arg{{Name: "name", Type: StringArg}}
}

func (a *APIFnProperty) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.GetProperty(a.prop)
	return
}
