// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements chain entry structures and functions

package holochain

import (
	"encoding/binary"
	"encoding/json"
	. "github.com/HC-Interns/holochain-proto/hash"
	"github.com/lestrrat/go-jsval"
	"io"
	"strings"
	"fmt"
	"errors"
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

// sysValidateEntry does system level validation for adding an entry (put or commit)
// It checks that entry is not nil, and that it conforms to the entry schema in the definition
// if it's a Links entry that the contents are correctly structured
// if it's a new agent entry, that identity matches the defined identity structure
// if it's a key that the structure is actually a public key
func sysValidateEntry(h *Holochain, def *EntryDef, entry Entry, pkg *Package) (err error) {
	switch def.Name {
	case DNAEntryType:
		err = ErrNotValidForDNAType
		return
	case KeyEntryType:
		b58pk, ok := entry.Content().(string)
		if !ok || !isValidPubKey(b58pk) {
			err = ValidationFailed(ValidationFailureBadPublicKeyFormat)
			return
		}
	case AgentEntryType:
		j, ok := entry.Content().(string)
		if !ok {
			err = ValidationFailedErr
			return
		}
		ae, _ := AgentEntryFromJSON(j)

		// check that the public key is unmarshalable
		if !isValidPubKey(ae.PublicKey) {
			err = ValidationFailed(ValidationFailureBadPublicKeyFormat)
			return err
		}

		// if there's a revocation, confirm that has a reasonable format
		if ae.Revocation != "" {
			revocation := &SelfRevocation{}
			err := revocation.Unmarshal(ae.Revocation)
			if err != nil {
				err = ValidationFailed(ValidationFailureBadRevocationFormat)
				return err
			}
		}

		// TODO check anything in the package
	case HeadersEntryType:
		// TODO check signatures!
	case DelEntryType:
		// TODO checks according to CRDT configuration?
	}

	if entry == nil {
		err = ValidationFailed(ErrNilEntryInvalid.Error())
		return
	}

	// see if there is a schema validator for the entry type and validate it if so
	if def.validator != nil {
		var input interface{}
		if def.DataFormat == DataFormatJSON {
			if err = json.Unmarshal([]byte(entry.Content().(string)), &input); err != nil {
				return
			}
		} else {
			input = entry
		}
		h.Debugf("Validating %v against schema", input)
		if err = def.validator.Validate(input); err != nil {
			err = ValidationFailed(err.Error())
			return
		}
		if def == DelEntryDef {
			// @TODO refactor and use in other sys types
			// @see https://github.com/holochain/holochain-proto/issues/733
			hashValue, ok := input.(map[string]interface{})["Hash"].(string)
			if !ok {
				err = ValidationFailed("expected string!")
				return
			}
			_, err = NewHash(hashValue)
			if err != nil {
				err = ValidationFailed(fmt.Sprintf("Error (%s) when decoding Hash value '%s'", err.Error(), hashValue))
				return
			}
		}
		if def == MigrateEntryDef {
			// @TODO refactor with above
			// @see https://github.com/holochain/holochain-proto/issues/733
			dnaHashValue, ok := input.(map[string]interface{})["DNAHash"].(string)
			if !ok {
				err = ValidationFailed("expected string!")
				return
			}
			_, err = NewHash(dnaHashValue)
			if err != nil {
				err = ValidationFailed(fmt.Sprintf("Error (%s) when decoding DNAHash value '%s'", err.Error(), dnaHashValue))
				return
			}

			keyValue, ok := input.(map[string]interface{})["Key"].(string)
			if !ok {
				err = ValidationFailed("expected string!")
				return
			}
			_, err = NewHash(keyValue)
			if err != nil {
				err = ValidationFailed(fmt.Sprintf("Error (%s) when decoding Key value '%s'", err.Error(), keyValue))
				return
			}

			typeValue, ok := input.(map[string]interface{})["Type"].(string)
			if !ok {
				err = ValidationFailed("expected string!")
				return
			}
			if !(typeValue == MigrateEntryTypeClose || typeValue == MigrateEntryTypeOpen) {
				err = ValidationFailed(fmt.Sprintf("Type value '%s' must be either '%s' or '%s'", typeValue, MigrateEntryTypeOpen, MigrateEntryTypeClose))
				return
			}
		}
	} else if def.DataFormat == DataFormatLinks {
		// Perform base validation on links entries, i.e. that all items exist and are of the right types
		// so first unmarshall the json, and then check that the hashes are real.
		var l struct{ Links []map[string]string }
		err = json.Unmarshal([]byte(entry.Content().(string)), &l)
		if err != nil {
			err = fmt.Errorf("invalid links entry, invalid json: %v", err)
			return
		}
		if len(l.Links) == 0 {
			err = errors.New("invalid links entry: you must specify at least one link")
			return
		}
		for _, link := range l.Links {
			h, ok := link["Base"]
			if !ok {
				err = errors.New("invalid links entry: missing Base")
				return
			}
			if _, err = NewHash(h); err != nil {
				err = fmt.Errorf("invalid links entry: Base %v", err)
				return
			}
			h, ok = link["Link"]
			if !ok {
				err = errors.New("invalid links entry: missing Link")
				return
			}
			if _, err = NewHash(h); err != nil {
				err = fmt.Errorf("invalid links entry: Link %v", err)
				return
			}
			_, ok = link["Tag"]
			if !ok {
				err = errors.New("invalid links entry: missing Tag")
				return
			}
		}

	}
	return
}
