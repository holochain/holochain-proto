package holochain

import (
	_"fmt"
	"testing"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/boltdb/bolt"

)

func TestNewBoltPersister(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	bp = NewBoltPersister(p).(*BoltPersister)
	Convey("It should create a struct",t,func(){
		So(bp.db,ShouldBeNil)
		So(bp.path,ShouldEqual,p)
	})
}

func TestBoltOpen(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	bp = NewBoltPersister(p).(*BoltPersister)
	defer cleanupTestDir(p)
	Convey("It should open the database for writing",t,func(){
		err := bp.Open()
		So(err,ShouldBeNil)
		So(fileExists(p),ShouldBeTrue)
	})
}

func TestBoltInit(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	bp = NewBoltPersister(p).(*BoltPersister)
	err := bp.Open()
	if err != nil {panic(err)}
	defer cleanupTestDir(p)
	Convey("It should initialize the database",t,func(){
		err = bp.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(MetaBucket))
			So(b,ShouldBeNil)
			return nil
		})
		err = bp.Init()
		So(err,ShouldBeNil)
		err = bp.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(MetaBucket))
			So(b,ShouldNotBeNil)
			return nil
		})

	})
}

func TestBoltGet(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	bp = NewBoltPersister(p).(*BoltPersister)
	err := bp.Init()
	if err != nil {panic(err)}
	defer cleanupTestDir(p)
	Convey("it should retrieve set data",t,func(){
		err = bp.PutMeta("fish",[]byte("cow"))
		So(err,ShouldBeNil)
		data,err := bp.GetMeta("fish")
		So(err,ShouldBeNil)
		So(string(data),ShouldEqual,"cow")
	})
}
