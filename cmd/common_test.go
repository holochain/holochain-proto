package cmd

import (
	"fmt"
	holo "github.com/metacurrency/holochain"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
)

func TestIsAppDir(t *testing.T) {
	Convey("it should test to see if dir is a holochain app", t, func() {

		d := holo.SetupTestDir()
		So(IsAppDir(d).Error(), ShouldEqual, "directory missing .hc subdirectory")
		err := os.MkdirAll(d+"/.hc", os.ModePerm)
		if err != nil {
			panic(err)
		}
		defer holo.CleanupTestDir(d)
		So(IsAppDir(d), ShouldBeNil)
	})
}

func TestGetService(t *testing.T) {
	d := holo.SetupTestDir()
	defer holo.CleanupTestDir(d)
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
	d := holo.SetupTestDir()
	defer holo.CleanupTestDir(d)

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
		d, service, h := holo.PrepareTestChain("test")
		defer holo.CleanupTestDir(d)

		// finally run the test.
		h, err := GetHolochain("test", service, "some-cmd")
		So(err, ShouldBeNil)
		So(h.Nucleus().DNA().Name, ShouldEqual, "test")
	})
}
