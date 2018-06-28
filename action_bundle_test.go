package holochain

import (
  "testing"
  . "github.com/smartystreets/goconvey/convey"
  . "github.com/HC-Interns/holochain-proto/hash"
)

func TestActionBundle(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)
	Convey("bundle action constructor should set timeout", t, func() {
		a := NewStartBundleAction(0, "myBundle")
		So(a.timeout, ShouldEqual, DefaultBundleTimeout)
		So(a.userParam, ShouldEqual, "myBundle")
		a = NewStartBundleAction(123, "myBundle")
		So(a.timeout, ShouldEqual, 123)
	})

	Convey("starting a bundle should set the bundle start point", t, func() {
		c := h.Chain()
		So(c.BundleStarted(), ShouldBeNil)
		a := NewStartBundleAction(100, "myBundle")
		_, err := a.Call(h)
		So(err, ShouldBeNil)
		So(c.BundleStarted().idx, ShouldEqual, c.Length()-1)
	})
	var hash Hash
	Convey("commit actions should commit to bundle after it's started", t, func() {
		So(h.chain.Length(), ShouldEqual, 2)
		So(h.chain.bundle.chain.Length(), ShouldEqual, 0)
		hash = commit(h, "oddNumbers", "99")

		So(h.chain.Length(), ShouldEqual, 2)
		So(h.chain.bundle.chain.Length(), ShouldEqual, 1)
	})
	Convey("but those commits should not show in the DHT", t, func() {
		_, _, _, _, err := h.dht.Get(hash, StatusDefault, GetMaskDefault)
		So(err, ShouldEqual, ErrHashNotFound)
	})

	Convey("closing a bundle should commit its entries to the chain", t, func() {
		So(h.chain.Length(), ShouldEqual, 2)
		a := &APIFnCloseBundle{commit: true}
		So(a.commit, ShouldEqual, true)
		_, err := a.Call(h)
		So(err, ShouldBeNil)
		So(h.chain.Length(), ShouldEqual, 3)
	})
	Convey("and those commits should now show in the DHT", t, func() {
		data, _, _, _, err := h.dht.Get(hash, StatusDefault, GetMaskDefault)
		So(err, ShouldBeNil)
		var e GobEntry
		err = e.Unmarshal(data)

		So(e.C, ShouldEqual, "99")
	})

	Convey("canceling a bundle should not commit entries to chain and should execute the bundleCanceled callback", t, func() {
		So(h.chain.Length(), ShouldEqual, 3)

		_, err := NewStartBundleAction(0, "debugit").Call(h)
		So(err, ShouldBeNil)
		commit(h, "oddNumbers", "7")

		a := &APIFnCloseBundle{commit: false}
		So(a.commit, ShouldEqual, false)
		ShouldLog(h.nucleus.alog, func() {
			_, err = a.Call(h)
			So(err, ShouldBeNil)
		}, `debug message during bundleCanceled with reason: userCancel`)
		So(h.chain.Length(), ShouldEqual, 3)
		So(h.chain.BundleStarted(), ShouldBeNil)
	})
	Convey("canceling a bundle should still commit entries if bundleCanceled returns BundleCancelResponseCommit", t, func() {
		So(h.chain.Length(), ShouldEqual, 3)

		_, err := NewStartBundleAction(0, "cancelit").Call(h)
		So(err, ShouldBeNil)
		commit(h, "oddNumbers", "7")
		a := &APIFnCloseBundle{commit: false}
		So(a.commit, ShouldEqual, false)
		ShouldLog(h.nucleus.alog, func() {
			_, err = a.Call(h)
			So(err, ShouldBeNil)
		}, `debug message during bundleCanceled: canceling cancel!`)
		So(h.chain.BundleStarted(), ShouldNotBeNil)
	})
}
