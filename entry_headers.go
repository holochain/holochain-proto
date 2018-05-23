package holochain

const (
  HeadersEntryType = SysEntryTypePrefix + "header"
)

const (
  HeadersEntrySchema = `
{
  "$id": "http://example.com/example.json",
  "type": "array",
  "definitions": {},
  "$schema": "http://json-schema.org/draft-07/schema#",
  "items": {
    "$id": "http://example.com/example.json/items",
    "type": "object",
    "properties": {
      "Source": {
        "$id": "http://example.com/example.json/items/properties/Source",
        "type": "string",
        "title": "The Source Schema ",
        "default": "",
        "examples": [
          "QmeLEGdTHwM4XYGggePJAYXLx968GiuiNooU1p7fa8T8zd"
        ]
      },
      "Header": {
        "$id": "http://example.com/example.json/items/properties/Header",
        "type": "object",
        "properties": {
          "Type": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/Type",
            "type": "string",
            "title": "The Type Schema ",
            "default": "",
            "examples": [
              "someType"
            ]
          },
          "Time": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/Time",
            "type": "string",
            "title": "The Time Schema ",
            "default": "",
            "examples": [
              "2018-03-15 19:30:05.740445736 -0400 EDT"
            ]
          },
          "EntryLink": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/EntryLink",
            "type": "string",
            "title": "The Entrylink Schema ",
            "default": "",
            "examples": [
              "QmeLEGdTHwM4XYGggePJAYXLx968GiuiNooU1p7fa8T8zd"
            ]
          },
          "HeaderLink": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/HeaderLink",
            "type": "string",
            "title": "The Headerlink Schema ",
            "default": "",
            "examples": [
              "QmWr1C3CeX12iZz98JGhzfsvfQpif29Ptwe86miZ9N9snU"
            ]
          },
          "TypeLink": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/TypeLink",
            "type": "string",
            "title": "The Typelink Schema ",
            "default": "",
            "examples": [
              "1"
            ]
          },
          "Signature": {
            "$id": "http://example.com/example.json/items/properties/Header/properties/Signature",
            "type": "string",
            "title": "The Signature Schema ",
            "default": "",
            "examples": [
              "StwmRCJtj9Ymjdo7ws8ZeNdmEi2GZzNdtbubT8MZBfxpXWQDLtQPDZWeSA2qHTsVtyN7tZCrYTeWmeCdcoYe197"
            ]
          }
        }
      },
      "Role": {
        "$id": "http://example.com/example.json/items/properties/Role",
        "type": "string",
        "title": "The Role Schema ",
        "default": "",
        "examples": [
          "someRole"
        ]
      }
    },
    "required": ["Header","Source"]
  }
}
`
)

var HeadersEntryDef = &EntryDef{Name: HeadersEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: HeadersEntrySchema}
