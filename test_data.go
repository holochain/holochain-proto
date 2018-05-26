package holochain

import (
	"crypto/rand"
	. "github.com/holochain/holochain-proto/hash"
	"github.com/multiformats/go-multihash"
)

// Generate a random Hash string for testing
func genTestStringHash() Hash {
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

// Generate a random string for testing
func genTestString() string {
	return string(genTestStringHash())
}

// Generate a random Header for testing
func genTestHeader() (header *Header, err error) {
	hashSpec, privKey, now := chainTestSetup()
	headerType := genTestString()
	entry := &GobEntry{C: genTestString()}
	prevHash := genTestStringHash()
	prevType := genTestStringHash()
	change := genTestStringHash()
	_, header, err = newHeader(hashSpec, now, headerType, entry, privKey, prevHash, prevType, change)

	return
}
