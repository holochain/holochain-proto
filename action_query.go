package holochain

import (
  "reflect"
)

//------------------------------------------------------------
// Query

type APIFnQuery struct {
	options *QueryOptions
}

func (a *APIFnQuery) Name() string {
	return "query"
}

func (a *APIFnQuery) Args() []Arg {
	return []Arg{{Name: "options", Type: MapArg, MapType: reflect.TypeOf(QueryOptions{}), Optional: true}}
}

func (a *APIFnQuery) Call(h *Holochain) (response interface{}, err error) {
	response, err = h.Query(a.options)
	return
}
