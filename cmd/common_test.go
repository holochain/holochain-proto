package cmd

import (
	"bytes"
	"fmt"
	holo "github.com/metacurrency/holochain"
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

func TestGetService(t *testing.T) {
	d := mkTestDirName()
	defer os.RemoveAll(d)
	Convey("it should fail to make a service if not initialized", t, func() {
		service, err := GetService(d)
		So(service, ShouldBeNil)
		So(err, ShouldEqual, ErrServiceUninitialized)
	})
	Convey("it should make a service once initialized", t, func() {
		holo.Init(d, holo.AgentName("test@example.com"))
		service, err := GetService(d)
		So(err, ShouldBeNil)
		So(service.Path, ShouldEqual, d)
	})
}

func TestGetHolochain(t *testing.T) {
	d := mkTestDirName()
	defer os.RemoveAll(d)

	Convey("it should fail when service not initialized", t, func() {
		h, err := GetHolochain("foobar", nil, "some-cmd")
		So(err, ShouldEqual, ErrServiceUninitialized)
		So(h, ShouldBeNil)
	})

	holo.Init(d, holo.AgentName("test@example.com"))
	service, _ := GetService(d)
	Convey("it should fail to get an non-existent holochain", t, func() {
		h, err := GetHolochain("foobar", service, "some-cmd")
		So(err.Error(), ShouldEqual, fmt.Sprintf("No DNA file in %s/foobar/dna/", d))
		So(h, ShouldBeNil)
	})

	Convey("it should get an installed holochain", t, func() {
		// build empty app template
		devPath := d + "/testdev"
		err := MakeDirs(devPath)
		if err != nil {
			panic(err)
		}
		scaffold := bytes.NewBuffer([]byte(holo.BasicTemplateScaffold))
		dna, err := holo.LoadDNAScaffold(scaffold)
		if err != nil {
			panic(err)
		}
		err = service.SaveDNAFile(devPath, dna, "json", false)
		if err != nil {
			panic(err)
		}

		// clone the template to be a real installed app
		var agent holo.Agent
		agent, err = holo.LoadAgent(d)
		err = service.Clone(devPath, d+"/test", agent, false)
		if err != nil {
			panic(err)
		}

		// finally run the test.
		h, err := GetHolochain("test", service, "some-cmd")
		So(err, ShouldBeNil)
		So(h.Nucleus().DNA().Name, ShouldEqual, "templateApp")
	})
}

func mkTestDirName() string {
	t := time.Now()
	d := "/tmp/holochain_test" + strconv.FormatInt(t.Unix(), 10) + "." + strconv.Itoa(t.Nanosecond())
	return d
}
