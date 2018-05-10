package holochain

import (
  "encoding/json"
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
    "Message": {
      "$id": "/properties/message",
      "type": "string",
      "title": "The Message Schema ",
      "default": ""
    }
  }
}
`
)

// OpenEntry struct holds the record of a source chain's opening
type OpenEntry struct {
	Hash Hash
	Message string
}

var OpenEntryDef = &EntryDef{Name: OpenEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: OpenEntrySchema}

func (e *OpenEntry) ToJSON() (encodedEntry string, err error) {
  var x struct {
    Message string
  }
  x.Message = e.Message
  var j []byte
  j, err = json.Marshal(x)
  encodedEntry = string(j)
  return
}

func OpenEntryFromJSON(j string) (entry OpenEntry, err error) {
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
