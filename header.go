// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements chain header structures & coding

package holochain

import (
	"bytes"
	"encoding/binary"
	ic "github.com/libp2p/go-libp2p-crypto"
	. "github.com/metacurrency/holochain/hash"
	"io"
	"time"
)

type Signature struct {
	S []byte
}

// StatusChange records change of status of an entry in the header
type StatusChange struct {
	Action string // either AddAction, ModAction, or DelAction
	Hash   Hash
}

// Header holds chain links, type, timestamp and signature
type Header struct {
	Type       string
	Time       time.Time
	HeaderLink Hash // link to previous headerq
	EntryLink  Hash // link to entry
	TypeLink   Hash // link to header of previous header of this type
	Sig        Signature
	Change     StatusChange
}

// newHeader makes Header object linked to a previous Header by hash
func newHeader(hashSpec HashSpec, now time.Time, t string, entry Entry, privKey ic.PrivKey, prev Hash, prevType Hash, change *StatusChange) (hash Hash, header *Header, err error) {
	var hd Header
	hd.Type = t
	now = now.Round(0)
	hd.Time = now
	hd.HeaderLink = prev
	hd.TypeLink = prevType
	if change != nil {
		hd.Change = *change
	} else {
		hd.Change.Hash = NullHash()
	}

	hd.EntryLink, err = entry.Sum(hashSpec)
	if err != nil {
		return
	}

	// sign the hash of the entry
	sig, err := privKey.Sign(hd.EntryLink.H)
	if err != nil {
		return
	}
	hd.Sig = Signature{S: sig}

	hash, _, err = (&hd).Sum(hashSpec)
	if err != nil {
		return
	}

	header = &hd
	return
}

// Sum encodes and creates a hash digest of the header
func (hd *Header) Sum(spec HashSpec) (hash Hash, b []byte, err error) {
	b, err = hd.Marshal()
	if err == nil {
		err = hash.Sum(spec, b)
	}
	return
}

// Marshal writes a header to bytes
func (hd *Header) Marshal() (b []byte, err error) {
	var s bytes.Buffer
	err = MarshalHeader(&s, hd)
	if err == nil {
		b = s.Bytes()
	}
	return
}

func writeStr(writer io.Writer, str string) (err error) {
	var b []byte
	b = []byte(str)
	l := uint8(len(b))
	err = binary.Write(writer, binary.LittleEndian, l)
	if err != nil {
		return
	}
	err = binary.Write(writer, binary.LittleEndian, b)
	return
}

// MarshalHeader writes a header to a binary stream
func MarshalHeader(writer io.Writer, hd *Header) (err error) {
	err = writeStr(writer, hd.Type)
	if err != nil {
		return
	}

	var b []byte
	b, err = hd.Time.MarshalBinary()
	err = binary.Write(writer, binary.LittleEndian, b)
	if err != nil {
		return
	}

	err = hd.HeaderLink.MarshalHash(writer)
	if err != nil {
		return
	}

	err = hd.EntryLink.MarshalHash(writer)
	if err != nil {
		return
	}

	err = hd.TypeLink.MarshalHash(writer)
	if err != nil {
		return
	}
	err = MarshalSignature(writer, &hd.Sig)
	if err != nil {
		return
	}

	err = writeStr(writer, hd.Change.Action)
	if err != nil {
		return
	}

	err = hd.Change.Hash.MarshalHash(writer)
	if err != nil {
		return
	}

	// write out 0 for future expansion (meta)
	z := uint64(0)
	err = binary.Write(writer, binary.LittleEndian, &z)
	if err != nil {
		return
	}
	return
}

// Unmarshal reads a header from bytes
func (hd *Header) Unmarshal(b []byte, hashSize int) (err error) {
	s := bytes.NewBuffer(b)
	err = UnmarshalHeader(s, hd, hashSize)
	return
}

func readStr(reader io.Reader) (str string, err error) {
	var l uint8
	err = binary.Read(reader, binary.LittleEndian, &l)
	if err != nil {
		return
	}

	var b = make([]byte, l)
	err = binary.Read(reader, binary.LittleEndian, b)
	if err != nil {
		return
	}
	str = string(b)
	return
}

// UnmarshalHeader reads a Header from a binary stream
func UnmarshalHeader(reader io.Reader, hd *Header, hashSize int) (err error) {

	hd.Type, err = readStr(reader)
	if err != nil {
		return
	}

	var b = make([]byte, 15)
	err = binary.Read(reader, binary.LittleEndian, b)
	if err != nil {
		return
	}
	hd.Time.UnmarshalBinary(b)

	err = hd.HeaderLink.UnmarshalHash(reader)
	if err != nil {
		return
	}

	err = hd.EntryLink.UnmarshalHash(reader)
	if err != nil {
		return
	}

	err = hd.TypeLink.UnmarshalHash(reader)
	if err != nil {
		return
	}

	err = UnmarshalSignature(reader, &hd.Sig)
	if err != nil {
		return
	}

	hd.Change.Action, err = readStr(reader)
	if err != nil {
		return
	}

	err = hd.Change.Hash.UnmarshalHash(reader)
	if err != nil {
		return
	}

	z := uint64(0)
	err = binary.Read(reader, binary.LittleEndian, &z)
	if err != nil {
		return
	}
	return
}

// MarshalSignature writes a signature to a binary stream
func MarshalSignature(writer io.Writer, s *Signature) (err error) {
	l := uint8(len(s.S))
	err = binary.Write(writer, binary.LittleEndian, l)
	if err != nil {
		return
	}
	err = binary.Write(writer, binary.LittleEndian, s.S)
	if err != nil {
		return
	}
	return
}

// UnmarshalSignature reads a Signature from a binary stream
func UnmarshalSignature(reader io.Reader, s *Signature) (err error) {
	var l uint8
	err = binary.Read(reader, binary.LittleEndian, &l)
	if err != nil {
		return
	}
	var b = make([]byte, l)
	err = binary.Read(reader, binary.LittleEndian, b)
	if err != nil {
		return
	}
	s.S = b
	return
}
