package holochain

import (
  "encoding/json"
  . "github.com/holochain/holochain-proto/hash"
)

const (
	MigrateToEntryType = SysEntryTypePrefix + "migrate-to"
	MigrateFromToEntryType = SysEntryTypePrefix + "migrate-from"
  // currently both to/from have the same schema
	MigrateEntrySchema = `
{
  "$id": "http://example.com/example.json",
  "type": "object",
  "definitions": {},
  "$schema": "http://json-schema.org/draft-07/schema#",
  "properties": {
    "Chain": {
      "$id": "/properties/Hash",
      "type": "string",
      "title": "The Hash Schema ",
      "default": ""
    },
    "User": {
      "$id": "/properties/Hash",
      "type": "string",
      "title": "The Hash Schema ",
      "default": ""
    },
    "Data": {
      "$id": "/properties/message",
      "type": "string",
      "title": "The Message Schema ",
      "default": ""
    }
  },
  "required": ["Chain" "User"]
}
`
)

func MigrateEntryToJSON(e *interface{}) (encodedEntry string, err error) {
  var x struct {
		Chain Hash
		User Hash
    Data string
	}
	x.Chain = e.Chain.String()
  x.User = e.User.String()
	x.Data = e.Data.String()
	var j []byte
	j, err = json.Marshal(x)
	encodedEntry = string(j)
	return
}

func MigrateEntryFromJSON(j string) (entry interface{}, err error) {
  var x struct {
		Chain Hash
    User Hash
		Data string
	}
	err = json.Unmarshal([]byte(j), &x)
	if err != nil {
		return
	}
	entry.Chain, err = NewHash(x.Chain)
	entry.User, err = NewHash(x.Hash)
  entry.Data = x.Data
	return
}

// MigrateToEntry struct is the record of a chain closing
type MigrateToEntry struct {
  Chain Hash
  User Hash
  Data string
}

var MigrateToEntryDef = &EntryDef{Name: MigrateToEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: MigrateEntrySchema}

func (e *MigrateToEntry) ToJSON() (encodedEntry string, err error) {
  return MigrateEntryToJSON(e)
}

func MigrateToEntryFromJSON(j string) (entry MigrateToEntry, err error) {
  return MigrateEntryFromJSON(j)
}

// MigrateFromEntry struct is the record of a chain referencing a closed chain
type MigrateFromEntry struct {
  Chain Hash
  User Hash
  Data string
}

var MigrateFromEntryDef = &EntryDef{Name: MigrateFromEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: MigrateEntrySchema}

func (e *MigrateFromEntry) ToJSON() (encodedEntry string, err error) {
  return MigrateEntryToJSON(e)
}

func MigrateFromEntryFromJSON(j string) (entry MigrateFromEntry, err error) {
  return MigrateEntryFromJSON(j)
}
