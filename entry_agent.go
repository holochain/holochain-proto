package holochain

const (
  AgentEntryType   = SysEntryTypePrefix + "agent"
)

// AgentEntry structure for building AgentEntryType entries
type AgentEntry struct {
	Identity   AgentIdentity
	Revocation string // marshaled revocation
	PublicKey  string // marshaled public key
}

const (
	AgentEntrySchema = `
{
  "$id": "http://example.com/example.json",
  "type": "object",
  "definitions": {},
  "$schema": "http://json-schema.org/draft-07/schema#",
  "properties": {
    "Identity": {
      "$id": "/properties/Identity",
      "type": "string",
      "title": "The Identity Schema ",
      "default": ""
    },
    "Revocation": {
      "$id": "/properties/Revocation",
      "type": "string",
      "title": "The Revocation Schema ",
      "default": ""
    },
    "PublicKey": {
      "$id": "/properties/PublicKey",
      "type": "string",
      "title": "The Publickey Schema ",
      "default": ""
    }
  },
  "required": ["Identity", "PublicKey"]
}`
)

var AgentEntryDef = &EntryDef{Name: AgentEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: AgentEntrySchema}

func (ae *AgentEntry) ToJSON() (encodedEntry string, err error) {
	var j []byte
	j, err = json.Marshal(ae)
	encodedEntry = string(j)
	return
}

func AgentEntryFromJSON(j string) (entry AgentEntry, err error) {
	err = json.Unmarshal([]byte(j), &entry)
	return
}
