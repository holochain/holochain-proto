package holochain

import(
	"strconv"
	"testing"
	"os"
	"time"
)


func ExpectErrString(t *testing.T,err error,text string) {
	if err.Error() != text {
		t.Error("expected '"+text+"' got",err)
	}
}

func ExpectNoErr(t *testing.T,err error) {
	if err != nil {
		t.Error("expected no err, got",err)
	}
}

func mkTestDirName() string {
	t := time.Now()
	d := "/tmp/holochain_test"+strconv.FormatInt(t.Unix(),10)+"."+strconv.Itoa(t.Nanosecond())
	return d
}

func setupTestService() (d string,s *Service) {
	d = mkTestDirName()
	agent := Agent("Herbert <h@bert.com>")
	s,err := Init(d,agent)
	if err != nil {panic(err)}
	return
}

func setupTestDir() string {
	d := mkTestDirName();
	err := os.MkdirAll(d,os.ModePerm)
	if err != nil {panic(err)}
	return d
}

func cleanupTestDir(path string) {
	func() {os.RemoveAll(path)}()
}
