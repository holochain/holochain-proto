package holochain

const (
  DNAEntryType = SysEntryTypePrefix + "dna"
  DataFormatSysDNA = "_DNA"
)

var DNAEntryDef = &EntryDef{Name: DNAEntryType, DataFormat: DataFormatSysDNA}
