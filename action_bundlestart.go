package holochain

//------------------------------------------------------------
// StartBundle

const (
	DefaultBundleTimeout = 5000
)

type APIFnStartBundle struct {
	timeout   int64
	userParam string
}

func NewStartBundleAction(timeout int, userParam string) *APIFnStartBundle {
	a := APIFnStartBundle{timeout: int64(timeout), userParam: userParam}
	if timeout == 0 {
		a.timeout = DefaultBundleTimeout
	}
	return &a
}

func (a *APIFnStartBundle) Name() string {
	return "bundleStart"
}

func (a *APIFnStartBundle) Args() []Arg {
	return []Arg{{Name: "timeout", Type: IntArg}, {Name: "userParam", Type: StringArg}}
}

func (a *APIFnStartBundle) Call(h *Holochain) (response interface{}, err error) {
	err = h.Chain().StartBundle(a.userParam)
	return
}
