package holochain

import(
  "testing"
  . "github.com/smartystreets/goconvey/convey"
  . "github.com/holochain/holochain-proto/hash"
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

func TestGenerateRandomString(t *testing.T) {
  Convey("random string should be unique", t, func() {
    n := 10
    a, err := generateRandomString(n)
    if err != nil {
      panic(err)
    }
    b, err := generateRandomString(n)
    if err != nil {
      panic(err)
    }
    So(a, ShouldNotEqual, b)
  })
}

func TestGenTestStringHash(t *testing.T) {
  Convey("random hash should be unique", t, func() {
    a, err := genTestStringHash()
    if err != nil {
      panic(err)
    }
    b, err := genTestStringHash()
    if err != nil {
      panic(err)
    }
    So(a, ShouldNotEqual, b)
  })

  Convey("random hash should roundtrip safely through strings", t, func() {
    a, err := genTestStringHash()
    if err != nil {
      panic(err)
    }
    s := a.String()
    roundtrip, err := NewHash(s)
    So(a, ShouldEqual, roundtrip)
  })
}

func TestGenTestMigrateEntry(t *testing.T) {
  entry, err := genTestMigrateEntry()
  if err != nil {
    panic(err)
  }

  Convey("generated test data in migrate entry should be unique", t, func() {
    So(entry.User, ShouldNotEqual, entry.Chain)
    So(entry.User, ShouldNotEqual, entry.Data)
    So(entry.Chain, ShouldNotEqual, entry.Data)
  })
}
