package holochain

import (
  . "github.com/holochain/holochain-proto/hash"
)

const (
	OpenEntryType = SysEntryTypePrefix + "open"
	OpenEntrySchema = `
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

// OpenEntry struct holds the record of a source chain's opening
type OpenEntry struct {
	Hash Hash
	Message string
}

var OpenEntryDef = &EntryDef{Name: OpenEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: OpenEntrySchema}
