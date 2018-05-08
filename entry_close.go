package holochain

import (
  . "github.com/holochain/holochain-proto/hash"
)

const (
	CloseEntryType = SysEntryTypePrefix + "close"
	CloseEntrySchema = SysMessageEntrySchema
)

// CloseEntry struct holds the record of a source chain's closure
type CloseEntry struct {
	Hash Hash
	Message string
}

var CloseEntryDef = &EntryDef{Name: CloseEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: CloseEntrySchema}

func (e *CloseEntry) ToJSON() (encodedEntry string, err error) {
	return SysMessageToJSON(e)
}

func CloseEntryFromJSON(j string) (entry CloseEntry, err error) {
	return SysMessageFromJSON(j)
}
