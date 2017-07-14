package holochain

import (
	"bytes"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"strconv"
	"strings"
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

func PrepareTestChain(n string) (d string, s *Service, h *Holochain) {
	return prepareTestChain(n)
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

func CleanupTestDir(path string) { cleanupTestDir(path) }

func cleanupTestDir(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		panic(err)
	}
}

func ShouldLog(log *Logger, message string, fn func()) {
	var buf bytes.Buffer
	w := log.w
	log.w = &buf
	e := log.Enabled
	log.Enabled = true
	fn()
	matched := strings.Index(buf.String(), message) >= 0
	if matched {
		So(matched, ShouldBeTrue)
	} else {
		So(buf.String(), ShouldEqual, message)
	}
	log.Enabled = e
	log.w = w
}

func compareFile(path1 string, path2 string, fileName string) bool {
	src, err := readFile(path1, fileName)
	if err != nil {
		panic(err)
	}
	dst, _ := readFile(path2, fileName)
	if err != nil {
		panic(err)
	}
	return (string(src) == string(dst)) && (string(src) != "")
}
