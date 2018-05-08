package holochain

import (
	. "github.com/holochain/holochain-proto/hash"
)

const (
	DelEntryType = SysEntryTypePrefix + "del"
	DelEntrySchema = SysMessageEntrySchema
)

// DelEntry struct holds the record of an entry's deletion
type DelEntry struct {
	Hash Hash
	Message string
}

var DelEntryDef = &EntryDef{Name: DelEntryType, DataFormat: DataFormatJSON, Sharing: Public, Schema: DelEntrySchema}

func (e *DelEntry) ToJSON() (encodedEntry string, err error) {
	return SysMessageToJSON(e)
}

func DelEntryFromJSON(j string) (entry DelEntry, err error) {
	return SysMessageFromJSON(j)
}
