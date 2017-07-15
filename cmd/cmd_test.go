package cmd

import (
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestIsAppDir(t *testing.T) {
	Convey("it should test to see if dir is a holochain app", t, func() {

		d := mkTestDirName()
		So(IsAppDir(d).Error(), ShouldEqual, "directory missing .hc subdirectory")
		err := os.MkdirAll(d+"/.hc", os.ModePerm)
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(d)
		So(IsAppDir(d), ShouldBeNil)
	})
}

func mkTestDirName() string {
	t := time.Now()
	d := "/tmp/holochain_test" + strconv.FormatInt(t.Unix(), 10) + "." + strconv.Itoa(t.Nanosecond())
	return d
}
