package holochain

import (
	_ "fmt"
	"github.com/boltdb/bolt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCreatePersister(t *testing.T) {
	Convey("should fail to create a persister based from bad type", t, func() {
		_, err := CreatePersister("non-existent-type", "/some/path")
		So(err.Error(), ShouldEqual, "Invalid persister name. Must be one of: bolt")
	})
	Convey("should create a validator based from a good schema type", t, func() {
		p := "/tmp/boltdb"
		v, err := CreatePersister(BoltPersisterName, p)
		bp := v.(*BoltPersister)
		So(err, ShouldBeNil)
		So(bp.path, ShouldEqual, p)
	})
}

func TestNewBoltPersister(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	v, _ := NewBoltPersister(p)
	bp = v.(*BoltPersister)
	Convey("It should create a struct", t, func() {
		So(bp.db, ShouldBeNil)
		So(bp.path, ShouldEqual, p)
	})
}

func TestBoltOpen(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	v, _ := CreatePersister(BoltPersisterName, p)
	bp = v.(*BoltPersister)
	defer cleanupTestDir(p)
	Convey("It should open the database for writing", t, func() {
		err := bp.Open()
		So(err, ShouldBeNil)
		So(fileExists(p), ShouldBeTrue)
	})
}

func TestBoltInit(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	v, _ := CreatePersister(BoltPersisterName, p)
	bp = v.(*BoltPersister)
	err := bp.Open()
	if err != nil {
		panic(err)
	}
	defer cleanupTestDir(p)
	Convey("It should initialize the database", t, func() {
		err = bp.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(MetaBucket))
			So(b, ShouldBeNil)
			return nil
		})
		err = bp.Init()
		So(err, ShouldBeNil)
		err = bp.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(MetaBucket))
			So(b, ShouldNotBeNil)
			return nil
		})

	})
}

func TestBoltGet(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	v, _ := CreatePersister(BoltPersisterName, p)
	bp = v.(*BoltPersister)
	err := bp.Init()
	if err != nil {
		panic(err)
	}
	defer cleanupTestDir(p)
	Convey("it should retrieve set data", t, func() {
		err = bp.PutMeta("fish", []byte("cow"))
		So(err, ShouldBeNil)
		data, err := bp.GetMeta("fish")
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "cow")
	})
}
