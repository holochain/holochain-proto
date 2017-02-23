// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements chain entry structures and functions

package holochain

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"github.com/lestrrat/go-jsschema"
	"github.com/lestrrat/go-jsval"
	"github.com/lestrrat/go-jsval/builder"
)

const (
	DNAEntryType = "_dna"
	KeyEntryType = "_key"
)

// EntryDef struct holds an entry definition
type EntryDef struct {
	Name       string
	DataFormat string
	Schema     string // file name of schema or language schema directive
	SchemaHash Hash
	validator  SchemaValidator
}

// interface for Entry serialization and deserialziation
type Entry interface {
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Content() interface{}
}

// interface for schema validation
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

// ByteEncoder encodes anything using gob
func ByteEncoder(data interface{}) (b []byte, err error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(data)
	if err != nil {
		return
	}
	b = buf.Bytes()
	return
}

// ByteEncoder decodes data encoded by ByteEncoder
func ByteDecoder(b []byte, to interface{}) (err error) {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(to)
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
