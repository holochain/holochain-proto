package holochain

import (
	"crypto/rand"
	. "github.com/holochain/holochain-proto/hash"
	"github.com/multiformats/go-multihash"
)

// Generate a random Hash string
func genTestHashString() Hash {
	randBytes := make([]byte, 64)
	_, err := rand.Read(randBytes)
	if err != nil {
		panic(err)
	}
	mhash, err := multihash.EncodeName(randBytes, "sha256")
	if err != nil {
		panic(err)
	}
	hash, err := HashFromBytes(mhash)
	if err != nil {
		panic(err)
	}

	return Hash(hash.String())
}
