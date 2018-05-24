package holochain

import (
	"encoding/json"
	. "github.com/holochain/holochain-proto/hash"
)

const (
  DelEntryType = SysEntryTypePrefix + "del"
  DelEntrySchema = `
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

// DelEntry struct holds the record of an entry's deletion
type DelEntry struct {
	Hash Hash
	Message string
}

var DelEntryDef = &EntryDef{Name: DelEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: DelEntrySchema}

func (e *DelEntry) ToJSON() (encodedEntry string, err error) {
	var x struct {
		Hash string
		Message string
	}
	x.Hash = e.Hash.String()
	x.Message = e.Message
	var j []byte
	j, err = json.Marshal(x)
	encodedEntry = string(j)
	return
}

func DelEntryFromJSON(j string) (entry DelEntry, err error) {
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
