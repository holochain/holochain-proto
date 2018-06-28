package main


import (
  "fmt"
  "os"
  "path/filepath"

  "github.com/robertkrimen/otto"
  holo "github.com/HC-Interns/holochain-proto"
  apptest "github.com/HC-Interns/holochain-proto/apptest"
)


func ScenarioJS(jsr *holo.JSRibosome, service *holo.Service, zomeName string, rootPath string, devPath string, name string) func(otto.FunctionCall) otto.Value {
  return func(call otto.FunctionCall) otto.Value {
    n, _ := call.Argument(0).ToInteger()
    fn := call.Argument(1)
    var args []interface{}
    for i := 0; i < int(n); i++ {
      identity := "tester-" + string(i)
      ctxFn := func (call otto.FunctionCall) otto.Value {
        h, _ := getHolochain2(service, identity, rootPath, devPath, name, false)
        apptest.SetupForPureJSTest(h, false, []holo.BridgeApp{})
        agentFn := call.Argument(0)
        code, _ := agentFn.ToString()
        zome, err := h.GetZome(zomeName)
        holo.Setup(jsr, h, zome)
        result, err := agentFn.Call(otto.UndefinedValue())
        // vm := holo.GetVM(jsr)
        // code = "(" + code + ")()"
        // result, err := holo.RunWithTimers(vm, code)
        fmt.Println("code: " + code)
        fmt.Printf("result: %v", result)
        fmt.Println("err: ", err)
        return result
      }
      args = append(args, ctxFn)
    }
    fn.Call(otto.UndefinedValue(), args...)
    return otto.UndefinedValue()
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