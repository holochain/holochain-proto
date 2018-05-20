package holochain

import (
  "encoding/json"
  . "github.com/holochain/holochain-proto/hash"
)

const (
	MigrateEntryType = SysEntryTypePrefix + "migrate"
	MigrateEntrySchema = `
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

// MigrateEntry struct holds the record of a source chain's opening
type MigrateEntry struct {
	Hash Hash
	Message string
}

var MigrateEntryDef = &EntryDef{Name: MigrateEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: MigrateEntrySchema}

func (e *MigrateEntry) ToJSON() (encodedEntry string, err error) {
  var x struct {
    Message string
  }
  x.Message = e.Message
  var j []byte
  j, err = json.Marshal(x)
  encodedEntry = string(j)
  return
}

func MigrateEntryFromJSON(j string) (entry MigrateEntry, err error) {
  var x struct {
		Message string
	}
	err = json.Unmarshal([]byte(j), &x)
	if err != nil {
		return
	}
	entry.Message = x.Message
	return
}
