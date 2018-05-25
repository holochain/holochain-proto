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
  "required": ["Chain" "User"]
}
`
)

// MigrateEntry struct is the record of a chain opening or closing
type MigrateEntry struct {
	Type  string
	Chain Hash
	User  Hash
	Data  string
}

var MigrateEntryDef = &EntryDef{Name: MigrateEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: MigrateEntrySchema}

func (e *MigrateEntry) ToJSON() (encodedEntry string, err error) {
	var x struct {
		Chain Hash
		User  Hash
		Data  string
	}
	x.Chain = e.Chain
	x.User = e.User
	x.Data = e.Data
	var j []byte
	j, err = json.Marshal(x)
	encodedEntry = string(j)
	return
}

func MigrateEntryFromJSON(j string) (entry MigrateEntry, err error) {
	var x struct {
		Chain Hash
		User  Hash
		Data  string
	}
	err = json.Unmarshal([]byte(j), &x)
	if err != nil {
		return
	}
	entry.Chain = x.Chain
	entry.User = x.User
	entry.Data = x.Data
	return
}
