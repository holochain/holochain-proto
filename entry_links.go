package holochain

const (
	DataFormatLinks   = "links"
)

// LinksEntry holds one or more links
type LinksEntry struct {
	Links []Link
}

// Link structure for holding meta tagging of linking entry
type Link struct {
	LinkAction string // StatusAction (either AddAction or DelAction)
	Base       string // hash of entry (perhaps elsewhere) to which we are attaching the link
	Link       string // hash of entry being linked to
	Tag        string // tag
}

func (ae *LinksEntry) ToJSON() (encodedEntry string, err error) {
	var j []byte
	j, err = json.Marshal(ae)
	encodedEntry = string(j)
	return
}

func LinksEntryFromJSON(j string) (entry LinksEntry, err error) {
	err = json.Unmarshal([]byte(j), &entry)
	return
}
