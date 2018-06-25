package holochain

import (
	"encoding/json"
	. "github.com/maackle/holochain-proto/hash"
)

const (
	MigrateEntryType = SysEntryTypePrefix + "migrate"
	// currently both to/from have the same schema
	MigrateEntrySchema = `
{
  "$id": "http://example.com/example.json",
  "type": "object",
  "definitions": {},
  "$schema": "http://json-schema.org/draft-07/schema#",
  "properties": {
		"Type": {
			"$id": "/properties/Type",
			"type": "string",
			"title": "The Type Schema ",
			"default": ""
		},
    "DNAHash": {
      "$id": "/properties/DNAHash",
      "type": "string",
      "title": "The DNAHash Schema ",
      "default": ""
    },
    "Key": {
      "$id": "/properties/Key",
      "type": "string",
      "title": "The Key Schema ",
      "default": ""
    },
    "Data": {
      "$id": "/properties/Data",
      "type": "string",
      "title": "The Data Schema ",
      "default": ""
    }
  },
  "required": ["Type", "DNAHash", "Key"]
}
`

	// Type can only be one of two things... open or close
	MigrateEntryTypeClose = "close"
	MigrateEntryTypeOpen  = "open"
)

// MigrateEntry struct is the record of a chain opening or closing
type MigrateEntry struct {
	Type  string
	DNAHash Hash
	Key  Hash
	Data  string
}

var MigrateEntryDef = &EntryDef{Name: MigrateEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: MigrateEntrySchema}

// @see https://github.com/holochain/holochain-proto/issues/731
func (e *MigrateEntry) Def() *EntryDef {
	return MigrateEntryDef
}

func (e *MigrateEntry) ToJSON() (encodedEntry string, err error) {
	var x struct {
		Type string
		DNAHash string
		Key  string
		Data  string
	}
	x.Type = e.Type
	x.DNAHash = e.DNAHash.String()
	x.Key = e.Key.String()
	x.Data = e.Data
	var j []byte
	j, err = json.Marshal(x)
	encodedEntry = string(j)
	return
}

func MigrateEntryFromJSON(j string) (entry MigrateEntry, err error) {
	var x struct {
		Type  string
		DNAHash string
		Key  string
		Data  string
	}
	err = json.Unmarshal([]byte(j), &x)
	if err != nil {
		return
	}

	entry.Type = x.Type
	entry.DNAHash, err = NewHash(x.DNAHash)
	entry.Key, err = NewHash(x.Key)
	entry.Data = x.Data
	return
}
