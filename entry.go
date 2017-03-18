// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements chain entry structures and functions

package holochain

import (
	"encoding/binary"
	"encoding/json"
	"github.com/lestrrat/go-jsschema"
	"github.com/lestrrat/go-jsval"
	"github.com/lestrrat/go-jsval/builder"
	"io"
)

const (
	DNAEntryType   = "%dna"
	AgentEntryType = "%agent"
	MetaEntryType  = "%meta" // result from putmeta statements
	KeyEntryType   = "%%key" // virtual entry type, not actually on the chain
)

const (
	DataFormatJSON    = "json"
	DataFormatString  = "string"
	DataFormatRawJS   = "js"
	DataFormatRawZygo = "zygo"
)

// AgentEntry structure for building KeyEntryType entries
type AgentEntry struct {
	Name    AgentName
	KeyType KeytypeType
	Key     []byte // marshaled public key
}

// MetaEntry structure for holding meta tagging of putmeta
type MetaEntry struct {
	H   Hash   // original entry (perhaps elsewhere) which we are putting meta
	M   Hash   // hash of the meta-data (must be in our chain)
	Tag string // meta tag
}

// EntryDef struct holds an entry definition
type EntryDef struct {
	Name       string
	DataFormat string
	Schema     string // file name of schema or language schema directive
	SchemaHash Hash
	validator  SchemaValidator
}

// Entry describes serialization and deserialziation of entry data
type Entry interface {
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Content() interface{}
	Sum(s HashSpec) (hash Hash, err error)
}

// SchemaValidator interface for schema validation
type SchemaValidator interface {
	Validate(interface{}) error
}

// GobEntry is a structure for implementing Gob encoding of Entry content
type GobEntry struct {
	C interface{}
}

// JSONEntry is a structure for implementing JSON encoding of Entry content
type JSONEntry struct {
	C interface{}
}

// MarshalEntry serializes an entry to a writer
func MarshalEntry(writer io.Writer, e Entry) (err error) {
	var b []byte
	b, err = e.Marshal()
	l := uint64(len(b))
	err = binary.Write(writer, binary.LittleEndian, l)
	if err != nil {
		return
	}
	err = binary.Write(writer, binary.LittleEndian, b)
	return
}

// UnmarshalEntry unserializes an entry from a reader
func UnmarshalEntry(reader io.Reader) (e Entry, err error) {
	var l uint64
	err = binary.Read(reader, binary.LittleEndian, &l)
	if err != nil {
		return
	}
	var b = make([]byte, l)
	err = binary.Read(reader, binary.LittleEndian, b)
	if err != nil {
		return
	}

	var g GobEntry
	err = g.Unmarshal(b)

	e = &g
	return
}

// implementation of Entry interface with gobs

func (e *GobEntry) Marshal() (b []byte, err error) {
	b, err = ByteEncoder(&e.C)
	return
}
func (e *GobEntry) Unmarshal(b []byte) (err error) {
	err = ByteDecoder(b, &e.C)
	return
}

func (e *GobEntry) Content() interface{} { return e.C }

func (e *GobEntry) Sum(s HashSpec) (h Hash, err error) {
	// encode the entry into bytes
	m, err := e.Marshal()
	if err != nil {
		return
	}

	// calculate the entry's hash and store it in the header
	err = h.Sum(s, m)
	if err != nil {
		return
	}

	return
}

// implementation of Entry interface with JSON

func (e *JSONEntry) Marshal() (b []byte, err error) {
	j, err := json.Marshal(e.C)
	if err != nil {
		return
	}
	b = []byte(j)
	return
}
func (e *JSONEntry) Unmarshal(b []byte) (err error) {
	err = json.Unmarshal(b, &e.C)
	return
}
func (e *JSONEntry) Content() interface{} { return e.C }

type JSONSchemaValidator struct {
	v *jsval.JSVal
}

// implementation of SchemaValidator with JSONSchema

func (v *JSONSchemaValidator) Validate(entry interface{}) (err error) {
	err = v.v.Validate(entry)
	return
}

// BuildJSONSchemaValidator builds a validator in an EntryDef
func (d *EntryDef) BuildJSONSchemaValidator(path string) (err error) {
	var s *schema.Schema
	s, err = schema.ReadFile(path + "/" + d.Schema)
	if err != nil {
		return
	}

	b := builder.New()
	var v JSONSchemaValidator
	v.v, err = b.Build(s)
	if err == nil {
		v.v.SetName(d.Schema)
		d.validator = &v
	}
	return
}
