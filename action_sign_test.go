package holochain

import (
	b58 "github.com/jbenet/go-base58"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestActionSigning(t *testing.T) {
	d, _, h := PrepareTestChain("test")
	defer CleanupTestChain(h, d)

	privKey := h.agent.PrivKey()
	sig, err := privKey.Sign([]byte("3"))
	if err != nil {
		panic(err)
	}

	var b58sig string
	Convey("sign action should return a b58 encoded signature", t, func() {
		fn := &APIFnSign{[]byte("3")}
		result, err := fn.Call(h)
		So(err, ShouldBeNil)
		b58sig = result.(string)

		So(b58sig, ShouldEqual, b58.Encode(sig))
	})
	var pubKey string
	pubKey, err = h.agent.EncodePubKey()
	if err != nil {
		panic(err)
	}

	Convey("verify signture action should test a signature", t, func() {
		fn := &APIFnVerifySignature{b58signature: b58sig, data: string([]byte("3")), b58pubKey: pubKey}
		result, err := fn.Call(h)
		So(err, ShouldBeNil)
		So(result.(bool), ShouldBeTrue)
		fn = &APIFnVerifySignature{b58signature: b58sig, data: string([]byte("34")), b58pubKey: pubKey}
		result, err = fn.Call(h)
		So(err, ShouldBeNil)
		So(result.(bool), ShouldBeFalse)
	})
}
