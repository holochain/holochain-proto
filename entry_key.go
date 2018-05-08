package holochain

const (
  // virtual entry type, not actually on the chain
  KeyEntryType     = VirtualEntryTypePrefix + "key"

  DataFormatSysKey  = "_key"
)

var KeyEntryDef = &EntryDef{Name: KeyEntryType, DataFormat: DataFormatSysKey}
