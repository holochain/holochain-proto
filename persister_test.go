package holochain

import (
	"fmt"
	"github.com/boltdb/bolt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestCreatePersister(t *testing.T) {
	Convey("should fail to create a persister based from bad type", t, func() {
		_, err := CreatePersister("non-existent-type", "/some/path")
		So(err.Error(), ShouldEqual, "Invalid persister name. Must be one of: bolt")
	})
	Convey("should create a persister based from a good schema type", t, func() {
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

func TestBoltClose(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	v, _ := CreatePersister(BoltPersisterName, p)
	bp = v.(*BoltPersister)
	defer cleanupTestDir(p)
	Convey("It should close the database", t, func() {
		bp.Open()
		So(bp.DB(), ShouldNotBeNil)
		bp.Close()
		So(bp.DB(), ShouldBeNil)
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

func TestBoltPutGet(t *testing.T) {
	var bp *BoltPersister
	p := "/tmp/boltdb"
	v, _ := CreatePersister(BoltPersisterName, p)
	bp = v.(*BoltPersister)
	err := bp.Init()
	if err != nil {
		panic(err)
	}
	defer cleanupTestDir(p)

	Convey("it should put & get entries", t, func() {
		hhash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
		header := []byte("bogus header")
		ehash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
		entry := GobEntry{C: "bogus entry data"}
		m, err := entry.Marshal()
		if err != nil {
			panic(err)
		}
		err = bp.Put("myData", hhash, header, ehash, m)
		So(err, ShouldBeNil)

		data, err := bp.GetMeta(TopMetaKey)
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", data), ShouldEqual, fmt.Sprintf("%v", hhash.H))

		data, err = bp.GetMeta(TopMetaKey + "_myData")
		So(err, ShouldBeNil)
		So(fmt.Sprintf("%v", data), ShouldEqual, fmt.Sprintf("%v", hhash.H))

		gentry, err := bp.GetEntry(ehash)
		So(err, ShouldBeNil)
		So(gentry.(string), ShouldEqual, "bogus entry data")

		badhash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh4")
		gentry, err = bp.GetEntry(badhash)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "hash not found")
		So(gentry, ShouldBeNil)

	})

	Convey("it should put & get meta data", t, func() {
		err = bp.PutMeta("fish", []byte("cow"))
		So(err, ShouldBeNil)
		data, err := bp.GetMeta("fish")
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "cow")
	})
}
