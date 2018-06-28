package main


import (
  "fmt"
  "os"
  "path/filepath"

  "github.com/robertkrimen/otto"
  holo "github.com/HC-Interns/holochain-proto"
  apptest "github.com/HC-Interns/holochain-proto/apptest"
)

func Throw(vm *otto.Otto, str string) {
  value, _ := vm.Call("new Error", nil, str)
  panic(value)
}

func ScenarioJS(jsr *holo.JSRibosome, service *holo.Service, zomeName string, rootPath string, devPath string, name string) func(otto.FunctionCall) otto.Value {
  vm := holo.GetVM(jsr)
  undef := otto.UndefinedValue()
  return func(call otto.FunctionCall) otto.Value {
    n, _ := call.Argument(0).ToInteger()
    fn := call.Argument(1)
    var args []interface{}
    for i := 0; i < int(n); i++ {
      identity := "tester-" + string(i)
      h, _ := getHolochain2(service, identity, rootPath, devPath, name, false)
      // TODO: pass config through func call
      err := apptest.SetupForPureJSTest(h, 500, false, []holo.BridgeApp{})
      if err != nil {
        panic("cannot set up chain for pure test")
      }
      ctxFn := func (call otto.FunctionCall) otto.Value {
        agentFn := call.Argument(0)
        zome, err := h.GetZome(zomeName)
        holo.Setup(jsr, h, zome)
        result, err := agentFn.Call(undef)
        if err != nil {
          Throw(vm, err.Error())
        }
        return result
      }
      args = append(args, ctxFn)
    }
    result, err := fn.Call(undef, args...)
    if err != nil {
      Throw(vm, err.Error())
    }
    return result
  }
}


// FIXME: this is copied from hcdev.go!!

func getHolochain2(service *holo.Service, identity string, rootPath string, devPath string, name string, verbose bool) (h *holo.Holochain, err error) {
  // clear out the previous chain data that was copied from the last test/run
  err = os.RemoveAll(filepath.Join(rootPath, name))
  if err != nil {
    return
  }
  var agent holo.Agent
  agent, err = holo.LoadAgent(rootPath)
  if err != nil {
    return
  }

  if identity != "" {
    holo.SetAgentIdentity(agent, holo.AgentIdentity(identity))
  }

  fmt.Printf("Copying chain to: %s\n", rootPath)
  h, err = service.Clone(devPath, filepath.Join(rootPath, name), agent, holo.CloneWithSameUUID, holo.InitializeDB)
  if err != nil {
    return
  }
  h.Close()

  h, err = service.Load(name)
  if err != nil {
    return
  }
  if verbose {
    fmt.Printf("Identity: %s\n", h.Agent().Identity())
    fmt.Printf("NodeID: %s\n", h.NodeIDStr())
  }
  return
}