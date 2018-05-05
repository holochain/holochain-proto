package holochain

//------------------------------------------------------------
// Bridge

type APIFnBridge struct {
	token    string
	url      string
	zome     string
	function string
	args     interface{}
}

func (fn *APIFnBridge) Name() string {
	return "bridge"
}

func (fn *APIFnBridge) Args() []Arg {
	return []Arg{{Name: "app", Type: HashArg}, {Name: "zome", Type: StringArg}, {Name: "function", Type: StringArg}, {Name: "args", Type: ArgsArg}}
}

func (fn *APIFnBridge) Call(h *Holochain) (response interface{}, err error) {
	body := bytes.NewBuffer([]byte(fn.args.(string)))
	var resp *http.Response
	resp, err = http.Post(fmt.Sprintf("%s/bridge/%s/%s/%s", fn.url, fn.token, fn.zome, fn.function), "", body)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var b []byte
	b, err = ioutil.ReadAll(resp.Body)
	response = string(b)
	return
}
