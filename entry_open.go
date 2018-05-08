package holochain

import (
  . "github.com/holochain/holochain-proto/hash"
)

const (
	OpenEntryType = SysEntryTypePrefix + "open"
	OpenEntrySchema = SysMessageEntrySchema
)

// OpenEntry struct holds the record of a source chain's opening
type OpenEntry struct {
	Hash Hash
	Message string
}

var OpenEntryDef = &EntryDef{Name: OpenEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: OpenEntrySchema}

func (e *OpenEntry) ToJSON() (encodedEntry string, err error) {
	return SysMessageToJSON(e)
}

func OpenEntryFromJSON(j string) (entry OpenEntry, err error) {
	return SysMessageFromJSON(j)
}
