package holochain

import(
  "testing"
  . "github.com/smartystreets/goconvey/convey"
)

func TestGenerateRandomBytes(t *testing.T) {
  Convey("random bytes should be unique", t, func() {
    n := 10
    a, err := generateRandomBytes(n)
    if err != nil {
      panic(err)
    }
    b, err := generateRandomBytes(n)
    if err != nil {
      panic(err)
    }
    So(a, ShouldNotEqual, b)
  })
}
