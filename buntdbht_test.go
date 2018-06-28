package holochain

import (
	"fmt"
	. "github.com/HC-Interns/holochain-proto/hash"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/tidwall/buntdb"
	"path/filepath"
	"testing"
)

func TestBuntHTOpen(t *testing.T) {
	d := SetupTestDir()
	defer CleanupTestDir(d)

	Convey("It should initialize the data store", t, func() {
		f := filepath.Join(d, DHTStoreFileName)
		So(FileExists(f), ShouldBeFalse)
		ht := &BuntHT{}
		ht.Open(f)
		So(FileExists(f), ShouldBeTrue)
	})
}

func TestBuntHTPutGetModDel(t *testing.T) {
	d := SetupTestDir()
	defer CleanupTestDir(d)
	node, err := makeNode(1234, "")
	if err != nil {
		panic(err)
	}
	defer node.Close()

	var id = node.HashAddr
	hash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2")
	var idx int

	ht := &BuntHT{}
	f := filepath.Join(d, DHTStoreFileName)
	ht.Open(f)

	Convey("It should store and retrieve", t, func() {
		err := ht.Put(node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: hash}), "someType", hash, id, []byte("some value"), StatusLive)
		So(err, ShouldBeNil)
		idx, _ = ht.GetIdx()

		data, entryType, sources, status, err := ht.Get(hash, StatusLive, GetMaskAll)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, StatusLive)
		So(sources[0], ShouldEqual, id.Pretty())

		badhash, _ := NewHash("QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3")
		data, entryType, _, _, err = ht.Get(badhash, StatusLive, GetMaskDefault)
		So(entryType, ShouldEqual, "")
		So(err, ShouldEqual, ErrHashNotFound)
	})

	Convey("It should iterate", t, func() {
		hlist := make([]Hash, 0)
		ht.Iterate(func(hsh Hash) bool {
			hlist = append(hlist, hsh)
			return true
		})
		So(len(hlist), ShouldEqual, 1)
		So(hlist[0].String(), ShouldEqual, hash.String())
	})

	Convey("mod should move the hash to the modified status and record replacedBy link", t, func() {
		newhashStr := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh4"
		newhash, _ := NewHash(newhashStr)

		m := node.NewMessage(MOD_REQUEST, HoldReq{RelatedHash: hash, EntryHash: newhash})

		err := ht.Mod(m, hash, newhash)
		So(err, ShouldBeNil)
		data, entryType, _, status, err := ht.Get(hash, StatusAny, GetMaskAll)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, StatusModified)

		afterIdx, _ := ht.GetIdx()

		So(afterIdx-idx, ShouldEqual, 1)

		data, entryType, _, status, err = ht.Get(hash, StatusLive, GetMaskDefault)
		So(err, ShouldEqual, ErrHashNotFound)

		data, entryType, _, status, err = ht.Get(hash, StatusDefault, GetMaskDefault)
		So(err, ShouldEqual, ErrHashModified)
		// replaced by link gets returned in the data!!
		So(string(data), ShouldEqual, newhashStr)

		links, err := ht.GetLinks(hash, SysTagReplacedBy, StatusLive)
		So(err, ShouldBeNil)
		So(len(links), ShouldEqual, 1)
		So(links[0].H, ShouldEqual, newhashStr)
	})

	Convey("del should move the hash to the deleted status", t, func() {
		m := node.NewMessage(DEL_REQUEST, HoldReq{RelatedHash: hash})

		err := ht.Del(m, hash)
		So(err, ShouldBeNil)

		data, entryType, _, status, err := ht.Get(hash, StatusAny, GetMaskAll)
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "some value")
		So(entryType, ShouldEqual, "someType")
		So(status, ShouldEqual, StatusDeleted)

		afterIdx, _ := ht.GetIdx()

		So(afterIdx-idx, ShouldEqual, 2)

		data, entryType, _, status, err = ht.Get(hash, StatusLive, GetMaskDefault)
		So(err, ShouldEqual, ErrHashNotFound)

		data, entryType, _, status, err = ht.Get(hash, StatusDefault, GetMaskDefault)
		So(err, ShouldEqual, ErrHashDeleted)

	})
}

func TestBuntHTLinking(t *testing.T) {
	d := SetupTestDir()
	defer CleanupTestDir(d)
	node, err := makeNode(1234, "")
	if err != nil {
		panic(err)
	}
	defer node.Close()

	var id = node.HashAddr

	ht := &BuntHT{}
	f := filepath.Join(d, DHTStoreFileName)
	ht.Open(f)

	baseStr := "QmZcUPvPhD1Xvk6mwijYF8AfR3mG31S1YsEfHG4khrFPRr"
	base, err := NewHash(baseStr)
	if err != nil {
		panic(err)
	}
	linkingEntryHashStr := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh3"
	linkingEntryHash, _ := NewHash(linkingEntryHashStr)
	linkHash1Str := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh1"
	linkHash1, _ := NewHash(linkHash1Str)
	linkHash2Str := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqh2"
	//linkHash2, _ := NewHash(linkHash2Str)
	Convey("It should fail if hash doesn't exist", t, func() {
		err := ht.PutLink(nil, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldEqual, ErrHashNotFound)

		v, err := ht.GetLinks(base, "tag foo", StatusLive)
		So(v, ShouldBeNil)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	err = ht.Put(node.NewMessage(PUT_REQUEST, HoldReq{EntryHash: base}), "someType", base, id, []byte("some value"), StatusLive)
	if err != nil {
		panic(err)
	}

	// the message doesn't actually matter for this test because it only gets used later in gossiping
	fakeMsg := node.NewMessage(LINK_REQUEST, HoldReq{RelatedHash: linkHash1, EntryHash: linkingEntryHash})

	Convey("Low level should add linking events to buntdb", t, func() {
		err := ht.link(fakeMsg, baseStr, linkHash1Str, "link test", StatusLive)
		So(err, ShouldBeNil)
		err = ht.db.View(func(tx *buntdb.Tx) error {
			err = tx.Ascend("link", func(key, value string) bool {
				So(key, ShouldEqual, fmt.Sprintf(`link:%s:%s:link test`, baseStr, linkHash1Str))
				So(value, ShouldEqual, fmt.Sprintf(`[{"Status":%d,"Source":"%s","LinksEntry":"%s"}]`, StatusLive, id.Pretty(), linkingEntryHashStr))
				return true
			})
			return nil
		})

		err = ht.link(fakeMsg, baseStr, linkHash1Str, "link test", StatusDeleted)
		So(err, ShouldBeNil)
		err = ht.db.View(func(tx *buntdb.Tx) error {
			err = tx.Ascend("link", func(key, value string) bool {
				So(value, ShouldEqual, fmt.Sprintf(`[{"Status":%d,"Source":"%s","LinksEntry":"%s"},{"Status":%d,"Source":"%s","LinksEntry":"%s"}]`, StatusLive, id.Pretty(), linkingEntryHashStr, StatusDeleted, id.Pretty(), linkingEntryHashStr))
				return true
			})
			return nil
		})
	})

	Convey("It should store and retrieve links values on a base", t, func() {
		data, err := ht.GetLinks(base, "tag foo", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 0)

		err = ht.PutLink(fakeMsg, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)

		err = ht.PutLink(fakeMsg, baseStr, linkHash2Str, "tag foo")
		So(err, ShouldBeNil)

		err = ht.PutLink(fakeMsg, baseStr, linkHash1Str, "tag bar")
		So(err, ShouldBeNil)

		data, err = ht.GetLinks(base, "tag foo", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 2)
		m := data[0]

		So(m.H, ShouldEqual, linkHash1Str)
		m = data[1]
		So(m.H, ShouldEqual, linkHash2Str)

		data, err = ht.GetLinks(base, "tag bar", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)
		So(data[0].H, ShouldEqual, linkHash1Str)
	})

	Convey("It should store and retrieve a links source", t, func() {
		err = ht.PutLink(fakeMsg, baseStr, linkHash1Str, "tag source")
		So(err, ShouldBeNil)

		data, err := ht.GetLinks(base, "tag source", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)

		data, err = ht.GetLinks(base, "tag source", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)
		So(data[0].Source, ShouldEqual, id.Pretty())
	})

	Convey("It should work to put a link a second time", t, func() {
		err = ht.PutLink(fakeMsg, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)
	})

	Convey("It should fail delete links non existent links bases and tags", t, func() {
		badHashStr := "QmY8Mzg9F69e5P9AoQPYat655HEhc1TVGs11tmfNSzkqhX"

		err := ht.DelLink(fakeMsg, badHashStr, linkHash1Str, "tag foo")
		So(err, ShouldEqual, ErrHashNotFound)
		err = ht.DelLink(fakeMsg, baseStr, badHashStr, "tag foo")
		So(err, ShouldEqual, ErrLinkNotFound)
		err = ht.DelLink(fakeMsg, baseStr, linkHash1Str, "tag baz")
		So(err, ShouldEqual, ErrLinkNotFound)
	})

	Convey("It should delete links", t, func() {
		err := ht.DelLink(fakeMsg, baseStr, linkHash1Str, "tag bar")
		So(err, ShouldBeNil)
		data, err := ht.GetLinks(base, "tag bar", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 0)

		err = ht.DelLink(fakeMsg, baseStr, linkHash1Str, "tag foo")
		So(err, ShouldBeNil)
		data, err = ht.GetLinks(base, "tag foo", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 1)

		err = ht.DelLink(fakeMsg, baseStr, linkHash2Str, "tag foo")
		So(err, ShouldBeNil)
		data, err = ht.GetLinks(base, "tag foo", StatusLive)
		So(err, ShouldBeNil)
		So(len(data), ShouldEqual, 0)
	})
}
