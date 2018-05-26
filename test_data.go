package holochain

import (
	"crypto/rand"
	"encoding/base64"
	. "github.com/holochain/holochain-proto/hash"
	"github.com/multiformats/go-multihash"
)

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
// @see https://blog.questionable.services/article/generating-secure-random-numbers-crypto-rand/
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRandomString returns a URL-safe, base64 encoded
// securely generated random string.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
// @see https://blog.questionable.services/article/generating-secure-random-numbers-crypto-rand/
func generateRandomString(s int) (string, error) {
	b, err := generateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b), err
}

// Generate a random Hash string for testing
func genTestStringHash() Hash {
	randBytes, err := generateRandomBytes(32)
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
	return genTestStringHash().String()
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
