package holochain

import (
  . "github.com/holochain/holochain-proto/hash"
)

const (
	CloseEntryType = SysEntryTypePrefix + "close"
	CloseEntrySchema = `
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

// CloseEntry struct holds the record of a source chain's closure
type CloseEntry struct {
	Hash Hash
	Message string
}

var CloseEntryDef = &EntryDef{Name: CloseEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: CloseEntrySchema}
