package holochain

import (
  "encoding/json"
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

// CloseEntry struct holds the record of a source chain's closure
type CloseEntry struct {
	Hash Hash
	Message string
}

var CloseEntryDef = &EntryDef{Name: CloseEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: CloseEntrySchema}

func (e *CloseEntry) ToJSON() (encodedEntry string, err error) {
  var x struct {
    Message string
  }
  x.Message = e.Message
  var j []byte
  j, err = json.Marshal(x)
  encodedEntry = string(j)
  return
}

func CloseEntryFromJSON(j string) (entry CloseEntry, err error) {
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
