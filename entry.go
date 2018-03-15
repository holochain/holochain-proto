// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements chain entry structures and functions

package holochain

import (
	"encoding/binary"
	"encoding/json"
	. "github.com/Holochain/holochain-proto/hash"
	"github.com/lestrrat/go-jsval"
	"io"
	"strings"
)

const (
	SysEntryTypePrefix     = "%"
	VirtualEntryTypePrefix = "%%"

	// System defined entry types

	DNAEntryType     = SysEntryTypePrefix + "dna"
	AgentEntryType   = SysEntryTypePrefix + "agent"
	HeadersEntryType = SysEntryTypePrefix + "header"
	KeyEntryType     = VirtualEntryTypePrefix + "key" // virtual entry type, not actually on the chain

	// Entry type formats

	DataFormatLinks    = "links"
	DataFormatJSON     = "json"
	DataFormatString   = "string"
	DataFormatRawJS    = "js"
	DataFormatRawZygo  = "zygo"
	DataFormatSysDNA   = "_DNA"
	DataFormatSysAgent = "_agent"
	DataFormatSysKey   = "_key"

	// Entry sharing types

	Public  = "public"
	Partial = "partial"
	Private = "private"
)

// AgentEntry structure for building AgentEntryType entries
type AgentEntry struct {
	Identity   AgentIdentity
	Revocation []byte // marshaled revocation
	PublicKey  []byte // marshaled public key
}

// LinksEntry holds one or more links
type LinksEntry struct {
	Links []Link
}

// Link structure for holding meta tagging of linking entry
type Link struct {
	LinkAction string // StatusAction (either AddAction or DelAction)
	Base       string // hash of entry (perhaps elsewhere) to which we are attaching the link
	Link       string // hash of entry being linked to
	Tag        string // tag
}

// DelEntry struct holds the record of an entry's deletion
type DelEntry struct {
	Hash    Hash
	Message string
}

// EntryDef struct holds an entry definition
type EntryDef struct {
	Name       string
	DataFormat string
	Sharing    string
	Schema     string
	validator  SchemaValidator
}

var DNAEntryDef = &EntryDef{Name: DNAEntryType, DataFormat: DataFormatSysDNA}
var AgentEntryDef = &EntryDef{Name: AgentEntryType, DataFormat: DataFormatSysAgent}
var KeyEntryDef = &EntryDef{Name: KeyEntryType, DataFormat: DataFormatSysKey}

const (
	HeadersEntrySchema = `
{
  "$id": "http://example.com/example.json",
  "type": "array",
  "definitions": {},
  "$schema": "http://json-schema.org/draft-07/schema#",
  "items": {
    "$id": "http://example.com/example.json/items",
    "type": "object",
    "properties": {
      "Header": {
        "$id": "http://example.com/example.json/items/properties/Header",
        "type": "object",
        "properties": {
          "Type": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/Type",
            "type": "string",
            "title": "The Type Schema ",
            "default": "",
            "examples": [
              "evenNumbers"
            ]
          },
          "Time": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/Time",
            "type": "string",
            "title": "The Time Schema ",
            "default": "",
            "examples": [
              "1969-12-31 19:00:01.000000001 -0500 EST"
            ]
          },
          "EntryLink": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/EntryLink",
            "type": "string",
            "title": "The Entrylink Schema ",
            "default": "",
            "examples": [
              "QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuU2"
            ]
          },
          "HeaderLink": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/HeaderLink",
            "type": "string",
            "title": "The Headerlink Schema ",
            "default": "",
            "examples": [
              "QmNiCwBNA8MWDADTFVq1BonUEJbS2SvjAoNkZZrhEwcuUi"
            ]
          },
          "TypeLink": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/TypeLink",
            "type": "string",
            "title": "The Typelink Schema ",
            "default": "",
            "examples": [
              "1"
            ]
          },
          "Signature": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/Signature",
            "type": "string",
            "title": "The Signature Schema ",
            "default": "",
            "examples": [
              "3eDinUfqsX4V2iuwFvFNSwyy4KEugYj6DPpssjrAsabkVvozBrWrLJRuA9AXhiN8R3MzZvyLfW2BV8zKDevSDiVR"
            ]
          }
        }
      },
      "Role": {
        "$id": "http://example.com/example.json/items/properties/Role",
        "type": "string",
        "title": "The Role Schema ",
        "default": "",
        "examples": [
          "someRole"
        ]
      }
    }
  }
}
`
)

var HeadersEntryDef = &EntryDef{Name: HeadersEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: HeadersEntrySchema}

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
