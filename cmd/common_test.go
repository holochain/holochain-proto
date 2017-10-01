package cmd

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	holo "github.com/metacurrency/holochain"
)

func TestIsAppDir(t *testing.T) {
	for _, configExtension := range GetConfigExtensionList() {
		Convey(fmt.Sprintf("it should test to see if dir is a holochain app (%v)", configExtension), t, func() {

			d, s := holo.SetupTestService()

			So(IsAppDir(d).Error(), ShouldEqual, "HC: Holochain App directory missing dna/dna.xyz config file")
			h, err := s.MakeTestingApp(filepath.Join(s.Path, "test"), configExtension, holo.InitializeDB, holo.CloneWithNewUUID, nil)
			if err != nil {
				panic(err)
			}
			So(IsAppDir(h.RootPath()), ShouldBeNil)
			holo.CleanupTestDir(d)
		})
	}

}

func TestGetHolochainRoot(t *testing.T) {
	os.Setenv("HOLOPATH", "some/dir/in/the/env")
	Convey("it should return the value passed in if not empty", t, func() {
		root, err := GetHolochainRoot("some/dir")
		So(err, ShouldBeNil)
		So(root, ShouldEqual, "some/dir")
	})
	Convey("it should get the root from the env", t, func() {
		root, err := GetHolochainRoot("")
		So(err, ShouldBeNil)
		So(root, ShouldEqual, "some/dir/in/the/env")
	})
	Convey("it should return the default directory if env and input are empty", t, func() {
		os.Setenv("HOLOPATH", "")
		root, err := GetHolochainRoot("")
		So(err, ShouldBeNil)
		u, err := user.Current()
		if err != nil {
			panic(err)
		}
		So(root, ShouldEqual, filepath.Join(u.HomeDir, holo.DefaultDirectoryName))
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
		holo.Init(d, holo.AgentIdentity("test@example.com"), nil)
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

	holo.Init(d, holo.AgentIdentity("test@example.com"), nil)
	service, _ := GetService(d)
	Convey("it should fail to get an non-existent holochain", t, func() {
		h, err := GetHolochain("foobar", service, "some-cmd")
		So(err.Error(), ShouldEqual, fmt.Sprintf("No DNA file in %s/foobar/dna/", d))
		So(h, ShouldBeNil)
	})

	Convey("it should get an installed holochain", t, func() {
		d, service, h := holo.PrepareTestChain("test")
		defer holo.CleanupTestChain(h, d)

		// finally run the test.
		h, err := GetHolochain("test", service, "some-cmd")
		So(err, ShouldBeNil)
		So(h.Nucleus().DNA().Name, ShouldEqual, "test")
		So(h.DHT(), ShouldNotBeNil)
		So(h.Node(), ShouldNotBeNil)
	})
}

func Test_OsExecFunctions_IsFile(t *testing.T) {
	d := holo.MakeTestDirName()
	os.MkdirAll(d, 0770)
	defer holo.CleanupTestDir(d)

	testFile := filepath.Join(d, "common_test.go.Test_OsExecPipes.aFile")

	Convey("it should when there is no touched file", t, func() {
		So(IsFile(testFile), ShouldEqual, false)
	})

	Convey("it should when there is a touched file", t, func() {
		OsExecPipes("touch", testFile)
		So(IsFile(testFile), ShouldEqual, true)
		OsExecSilent("rm", testFile)
		So(IsFile(testFile), ShouldEqual, false)
	})
}

func Test_TimestampFunctions(t *testing.T) {
	Convey("check second adder", t, func() {
		now := time.Now().Unix()
		So(GetUnixTimestamp_secondsFromNow(10), ShouldBeGreaterThanOrEqualTo, now+10)
	})
	Convey("check duration from now timestamp", t, func() {
		targetTime := GetUnixTimestamp_secondsFromNow(10)
		durationUntil := GetDuration_fromUnixTimestamp(targetTime)
		// fmt.Printf("now, target, durationUntil: %v, %v, %v\n\n", time.Now(), targetTime, durationUntil)
		So(time.Now().Add(durationUntil).After(time.Now().Add(8*time.Second)), ShouldEqual, true)
		So(time.Now().Add(durationUntil).Before(time.Now().Add(12*time.Second)), ShouldEqual, true)
	})

}
