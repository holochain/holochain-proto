package holochain

//------------------------------------------------------------
// Call

type APIFnCall struct {
	zome     string
	function string
	args     interface{}
}

func (fn *APIFnCall) Name() string {
	return "call"
}

func (fn *APIFnCall) Args() []Arg {
	return []Arg{{Name: "zome", Type: StringArg}, {Name: "function", Type: StringArg}, {Name: "args", Type: ArgsArg}}
}

func (fn *APIFnCall) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.Call(fn.zome, fn.function, fn.args, ZOME_EXPOSURE)
	return
}
