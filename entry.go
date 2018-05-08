// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements chain entry structures and functions

package holochain

import (
	"encoding/binary"
	"encoding/json"
	. "github.com/holochain/holochain-proto/hash"
	"github.com/lestrrat/go-jsval"
	"io"
	"strings"
)

const (
	SysEntryTypePrefix     = "%"
	VirtualEntryTypePrefix = "%%"

	// Entry type formats
	DataFormatJSON    = "json"
	DataFormatString  = "string"
	DataFormatRawJS   = "js"
	DataFormatRawZygo = "zygo"

	// Entry sharing types

	Public  = "public"
	Partial = "partial"
	Private = "private"

	// Standard schemas
	SysMessageEntrySchema = `
{
  "$id": "http://example.com/example.json",
  "type": "object",
  "definitions": {},
  "$schema": "http://json-schema.org/draft-07/schema#",
  "properties": {
    "Hash": {
      "$id": "/properties/Hash",
      "type": "string",
      "title": "The Hash Schema ",
      "default": ""
    },
    "Message": {
      "$id": "/properties/message",
      "type": "string",
      "title": "The Message Schema ",
      "default": ""
    }
  },
  "required": ["Hash"]
}
`
)



// EntryDef struct holds an entry definition
type EntryDef struct {
	Name       string
	DataFormat string
	Sharing    string
	Schema     string
	validator  SchemaValidator
}

func (def EntryDef) isSharingPublic() bool {
	return def.Sharing == Public || def.DataFormat == DataFormatLinks
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

// IsSysEntry returns true if the entry type is system defined
func (def *EntryDef) IsSysEntry() bool {
	return strings.HasPrefix(def.Name, SysEntryTypePrefix)
}

// IsVirtualEntry returns true if the entry type is virtual
func (def *EntryDef) IsVirtualEntry() bool {
	return strings.HasPrefix(def.Name, VirtualEntryTypePrefix)
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
	h, err = Sum(s, m)
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
	validator, err := BuildJSONSchemaValidatorFromFile(path)
	if err != nil {
		return
	}
	validator.v.SetName(d.Name)
	d.validator = validator
	return
}

func (d *EntryDef) BuildJSONSchemaValidatorFromString(schema string) (err error) {
	validator, err := BuildJSONSchemaValidatorFromString(schema)
	if err != nil {
		return
	}
	validator.v.SetName(d.Name)
	d.validator = validator
	return
}

func SysMessageToJSON (interface{}) (encodedEntry string, err error) {
	var x struct {
		Hash    string
		Message string
	}
	x.Hash = e.Hash.String()
	x.Message = e.Message
	var j []byte
	j, err = json.Marshal(x)
	encodedEntry = string(j)
	return
}

func SysMessageFromJSON (j string) (entry interface{}, err error) {
	var x struct {
		Hash    string
		Message string
	}
	err = json.Unmarshal([]byte(j), &x)
	if err != nil {
		return
	}
	entry.Message = x.Message
	entry.Hash, err = NewHash(x.Hash)
	return
}
