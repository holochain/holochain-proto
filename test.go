package holochain

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func ExpectNoErr(t *testing.T, err error) {
	if err != nil {
		t.Error("expected no err, got", err)
	}
}

func mkTestDirName() string {
	t := time.Now()
	d := "/tmp/holochain_test" + strconv.FormatInt(t.Unix(), 10) + "." + strconv.Itoa(t.Nanosecond())
	return d
}

func setupTestService() (d string, s *Service) {
	d = mkTestDirName()
	agent := AgentID("Herbert <h@bert.com>")
	s, err := Init(d, agent)
	if err != nil {
		panic(err)
	}
	return
}

func setupTestChain(n string) (d string, s *Service, h *Holochain) {
	d, s = setupTestService()
	path := s.Path + "/" + n
	h, err := GenDev(path)
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
