package holochain

import (
	"encoding/json"
	. "github.com/holochain/holochain-proto/hash"
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
    "Chain": {
      "$id": "/properties/Chain",
      "type": "string",
      "title": "The Chain Schema ",
      "default": ""
    },
    "User": {
      "$id": "/properties/User",
      "type": "string",
      "title": "The User Schema ",
      "default": ""
    },
    "Data": {
      "$id": "/properties/Data",
      "type": "string",
      "title": "The Data Schema ",
      "default": ""
    }
  },
  "required": ["Type", "Chain", "User"]
}
`

	// Type can only be one of two things... open or close
	MigrateEntryTypeClose = "close"
	MigrateEntryTypeOpen  = "open"
)

// MigrateEntry struct is the record of a chain opening or closing
type MigrateEntry struct {
	Type  string
	Chain Hash
	User  Hash
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
		Chain string
		User  string
		Data  string
	}
	x.Type = e.Type
	x.Chain = e.Chain.String()
	x.User = e.User.String()
	x.Data = e.Data
	var j []byte
	j, err = json.Marshal(x)
	encodedEntry = string(j)
	return
}

func MigrateEntryFromJSON(j string) (entry MigrateEntry, err error) {
	var x struct {
		Type  string
		Chain string
		User  string
		Data  string
	}
	err = json.Unmarshal([]byte(j), &x)
	if err != nil {
		return
	}

	entry.Type = x.Type
	entry.Chain, err = NewHash(x.Chain)
	entry.User, err = NewHash(x.User)
	entry.Data = x.Data
	return
}
