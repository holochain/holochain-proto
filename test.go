package holochain

import (
	"os"
	"strconv"
	"time"
)

var Crash bool

func Panix(on string) {
	if Crash {
		panic(on)
	}
}

func mkTestDirName() string {
	t := time.Now()
	d := "/tmp/holochain_test" + strconv.FormatInt(t.Unix(), 10) + "." + strconv.Itoa(t.Nanosecond())
	return d
}

func setupTestService() (d string, s *Service) {
	d = mkTestDirName()
	agent := AgentName("Herbert <h@bert.com>")
	s, err := Init(d+"/"+DefaultDirectoryName, agent)
	s.Settings.DefaultBootstrapServer = "localhost:3142"
	if err != nil {
		panic(err)
	}
	return
}

func setupTestChain(n string) (d string, s *Service, h *Holochain) {
	d, s = setupTestService()
	path := s.Path + "/" + n
	h, err := s.GenDev(path, "toml")
	if err != nil {
		panic(err)
	}
	return
}

func prepareTestChain(n string) (d string, s *Service, h *Holochain) {
	d, s, h = setupTestChain("test")
	_, err := h.GenChain()
	if err != nil {
		panic(err)
	}
	err = h.Activate()
	if err != nil {
		panic(err)
	}
	return
}

func setupTestDir() string {
	d := mkTestDirName()
	err := os.MkdirAll(d, os.ModePerm)
	if err != nil {
		panic(err)
	}
	return d
}

func cleanupTestDir(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		panic(err)
	}
}
