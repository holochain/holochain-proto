// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// implements a buntdb based instance of HashTable

package holochain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	. "github.com/HC-Interns/holochain-proto/hash"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/tidwall/buntdb"
)

type BuntHT struct {
	db *buntdb.DB
}

// linkEvent represents the value stored in buntDB associated with a
// link key for one source having stored one LinkingEntry
// (The Link struct defined in entry.go is encoded in the key used for buntDB)
type linkEvent struct {
	Status     int
	Source     string
	LinksEntry string
}

func (ht *BuntHT) Open(options interface{}) (err error) {
	file := options.(string)
	db, err := buntdb.Open(file)
	if err != nil {
		panic(err)
	}
	db.CreateIndex("link", "link:*", buntdb.IndexString)
	db.CreateIndex("idx", "idx:*", buntdb.IndexInt)
	db.CreateIndex("peer", "peer:*", buntdb.IndexString)
	db.CreateIndex("list", "list:*", buntdb.IndexString)
	db.CreateIndex("entry", "entry:*", buntdb.IndexString)

	ht.db = db
	return
}

// Put stores a value to the DHT store
// N.B. This call assumes that the value has already been validated
func (ht *BuntHT) Put(m *Message, entryType string, key Hash, src peer.ID, value []byte, status int) (err error) {
	k := key.String()
	err = ht.db.Update(func(tx *buntdb.Tx) error {
		_, err := incIdx(tx, m)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("entry:"+k, string(value), nil)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("type:"+k, entryType, nil)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("src:"+k, peer.IDB58Encode(src), nil)
		if err != nil {
			return err
		}
		_, _, err = tx.Set("status:"+k, fmt.Sprintf("%d", status), nil)
		if err != nil {
			return err
		}
		return err
	})
	return
}

// Del moves the given hash to the StatusDeleted status
// N.B. this functions assumes that the validity of this action has been confirmed
func (ht *BuntHT) Del(m *Message, key Hash) (err error) {
	k := key.String()
	err = ht.db.Update(func(tx *buntdb.Tx) error {
		err = _setStatus(tx, m, k, StatusDeleted)
		return err
	})
	return
}

func _setStatus(tx *buntdb.Tx, m *Message, key string, status int) (err error) {

	_, err = tx.Get("entry:" + key)
	if err != nil {
		if err == buntdb.ErrNotFound {
			err = ErrHashNotFound
		}
		return
	}

	_, err = incIdx(tx, m)
	if err != nil {
		return
	}

	_, _, err = tx.Set("status:"+key, fmt.Sprintf("%d", status), nil)
	if err != nil {
		return
	}
	return
}

// Mod moves the given hash to the StatusModified status
// N.B. this functions assumes that the validity of this action has been confirmed
func (ht *BuntHT) Mod(m *Message, key Hash, newkey Hash) (err error) {
	k := key.String()
	err = ht.db.Update(func(tx *buntdb.Tx) error {
		err = _setStatus(tx, m, k, StatusModified)
		if err == nil {
			link := newkey.String()
			err = _link(tx, k, link, SysTagReplacedBy, m.From, StatusLive, newkey)
			if err == nil {
				_, _, err = tx.Set("replacedBy:"+k, link, nil)
				if err != nil {
					return err
				}
			}
		}
		return err
	})
	return
}

func _get(tx *buntdb.Tx, k string, statusMask int) (string, error) {
	val, err := tx.Get("entry:" + k)
	if err == buntdb.ErrNotFound {
		err = ErrHashNotFound
		return val, err
	}
	var statusVal string
	statusVal, err = tx.Get("status:" + k)
	if err == nil {

		if statusMask == StatusDefault {
			// if the status mask is not given (i.e. Default) then
			// we return information about the status if it's other than live
			switch statusVal {
			case StatusDeletedVal:
				err = ErrHashDeleted
			case StatusModifiedVal:
				val, err = tx.Get("replacedBy:" + k)
				if err != nil {
					panic("missing expected replacedBy record")
				}
				err = ErrHashModified
			case StatusRejectedVal:
				err = ErrHashRejected
			case StatusLiveVal:
			default:
				panic("unknown status!")
			}
		} else {
			// otherwise we return the value only if the status is in the mask
			var status int
			status, err = strconv.Atoi(statusVal)
			if err == nil {
				if (status & statusMask) == 0 {
					err = ErrHashNotFound
				}
			}
		}
	}
	return val, err
}

// Exists checks for the existence of the hash in the store
func (ht *BuntHT) Exists(key Hash, statusMask int) (err error) {
	err = ht.db.View(func(tx *buntdb.Tx) error {
		_, err := _get(tx, key.String(), statusMask)
		return err
	})
	return
}

// Source returns the source node address of a given hash
func (ht *BuntHT) Source(key Hash) (id peer.ID, err error) {
	err = ht.db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get("src:" + key.String())
		if err == buntdb.ErrNotFound {
			err = ErrHashNotFound
		}
		if err == nil {
			id, err = peer.IDB58Decode(val)
		}
		return err
	})
	return
}

// Get retrieves a value from the DHT store
func (ht *BuntHT) Get(key Hash, statusMask int, getMask int) (data []byte, entryType string, sources []string, status int, err error) {
	if getMask == GetMaskDefault {
		getMask = GetMaskEntry
	}
	err = ht.db.View(func(tx *buntdb.Tx) error {
		k := key.String()
		val, err := _get(tx, k, statusMask)
		if err != nil {
			data = []byte(val) // gotta do this because value is valid if ErrHashModified
			return err
		}
		data = []byte(val)

		if (getMask & GetMaskEntryType) != 0 {
			entryType, err = tx.Get("type:" + k)
			if err != nil {
				return err
			}
		}
		if (getMask & GetMaskSources) != 0 {
			val, err = tx.Get("src:" + k)
			if err == buntdb.ErrNotFound {
				err = ErrHashNotFound
			}
			if err == nil {
				sources = append(sources, val)
			}
			if err != nil {
				return err
			}
		}

		val, err = tx.Get("status:" + k)
		if err != nil {
			return err
		}
		status, err = strconv.Atoi(val)
		if err != nil {
			return err
		}

		return err
	})
	return
}

// _link is a low level routine to add a link, also used by delLink
// this ensure monotonic recording of linking attempts
func _link(tx *buntdb.Tx, base string, link string, tag string, src peer.ID, status int, linkingEntryHash Hash) (err error) {
	key := "link:" + base + ":" + link + ":" + tag
	var val string
	val, err = tx.Get(key)
	source := peer.IDB58Encode(src)
	lehStr := linkingEntryHash.String()
	var records []linkEvent
	if err == nil {
		// load the previous value so we can append to it.
		json.Unmarshal([]byte(val), &records)

		// TODO: if the link exists, then load the statuses and see
		// what we should do about this situation
		/*
			// search for the source and linking entry in the status
			for _, s := range records {
				if s.Source == source && s.LinksEntry == lehStr {
					if status == StatusLive && s.Status != status {
						err = ErrPutLinkOverDeleted
						return
					}
					// return silently because this is just a duplicate putLink
					break
				}
			} // fall through and add this linking event.
		*/

	} else if err == buntdb.ErrNotFound {
		// when deleting the key must exist
		if status == StatusDeleted {
			err = ErrLinkNotFound
			return
		}
		err = nil
	} else {
		return
	}
	records = append(records, linkEvent{status, source, lehStr})
	var b []byte
	b, err = json.Marshal(records)
	if err != nil {
		return
	}
	_, _, err = tx.Set(key, string(b), nil)
	if err != nil {
		return
	}
	return
}

func (ht *BuntHT) link(m *Message, base string, link string, tag string, status int) (err error) {
	err = ht.db.Update(func(tx *buntdb.Tx) error {
		_, err := _get(tx, base, StatusLive)
		if err != nil {
			return err
		}
		err = _link(tx, base, link, tag, m.From, status, m.Body.(HoldReq).EntryHash)
		if err != nil {
			return err
		}

		//var index string
		_, err = incIdx(tx, m)

		if err != nil {
			return err
		}
		return nil
	})
	return
}

// PutLink associates a link with a stored hash
// N.B. this function assumes that the data associated has been properly retrieved
// and validated from the cource chain
func (ht *BuntHT) PutLink(m *Message, base string, link string, tag string) (err error) {
	err = ht.link(m, base, link, tag, StatusLive)
	return
}

// DelLink removes a link and tag associated with a stored hash
// N.B. this function assumes that the action has been properly validated
func (ht *BuntHT) DelLink(m *Message, base string, link string, tag string) (err error) {
	err = ht.link(m, base, link, tag, StatusDeleted)
	return
}

// GetLinks retrieves meta value associated with a base
func (ht *BuntHT) GetLinks(base Hash, tag string, statusMask int) (results []TaggedHash, err error) {
	b := base.String()
	err = ht.db.View(func(tx *buntdb.Tx) error {
		_, err := _get(tx, b, StatusLive+StatusModified) //only get links on live and modified bases
		if err != nil {
			return err
		}

		if statusMask == StatusDefault {
			statusMask = StatusLive
		}

		results = make([]TaggedHash, 0)
		err = tx.Ascend("link", func(key, value string) bool {
			x := strings.Split(key, ":")
			t := string(x[3])
			if string(x[1]) == b && (tag == "" || tag == t) {
				var records []linkEvent
				json.Unmarshal([]byte(value), &records)
				l := len(records)
				//TODO: this is totally bogus currently simply
				// looking at the last item we ever got
				if l > 0 {
					entry := records[l-1]
					if err == nil && (entry.Status&statusMask) > 0 {
						th := TaggedHash{H: string(x[2]), Source: entry.Source}
						if tag == "" {
							th.T = t
						}
						results = append(results, th)
					}
				}
			}

			return true
		})

		return err
	})
	return
}

// Close cleans up any resources used by the table
func (ht *BuntHT) Close() {
	ht.db.Close()
	ht.db = nil
}

// incIdx adds a new index record to dht for gossiping later
func incIdx(tx *buntdb.Tx, m *Message) (index string, err error) {
	// if message is nil we can't record this for gossiping
	// this should only be the case for the DNA
	if m == nil {
		return
	}

	var idx int
	idx, err = getIntVal("_idx", tx)
	if err != nil {
		return
	}
	idx++
	index = fmt.Sprintf("%d", idx)
	_, _, err = tx.Set("_idx", index, nil)
	if err != nil {
		return
	}

	var msg string

	if m != nil {
		var b []byte
		b, err = ByteEncoder(m)
		if err != nil {
			return
		}
		msg = string(b)
	}
	_, _, err = tx.Set("idx:"+index, msg, nil)
	if err != nil {
		return
	}

	f, err := m.Fingerprint()
	if err != nil {
		return
	}
	_, _, err = tx.Set("f:"+f.String(), index, nil)
	if err != nil {
		return
	}

	return
}

// getIntVal returns an integer value at a given key, and assumes the value 0 if the key doesn't exist
func getIntVal(key string, tx *buntdb.Tx) (idx int, err error) {
	var val string
	val, err = tx.Get(key)
	if err == buntdb.ErrNotFound {
		err = nil
	} else if err != nil {
		return
	} else {
		idx, err = strconv.Atoi(val)
		if err != nil {
			return
		}
	}
	return
}

// GetIdx returns the current index of changes to the HashTable
func (ht *BuntHT) GetIdx() (idx int, err error) {
	err = ht.db.View(func(tx *buntdb.Tx) error {
		var e error
		idx, e = getIntVal("_idx", tx)
		if e != nil {
			return e
		}
		return nil
	})
	return
}

// GetIdxMessage returns the messages that causes the change at a given index
func (ht *BuntHT) GetIdxMessage(idx int) (msg Message, err error) {
	err = ht.db.View(func(tx *buntdb.Tx) error {
		msgStr, e := tx.Get(fmt.Sprintf("idx:%d", idx))
		if e == buntdb.ErrNotFound {
			return ErrNoSuchIdx
		}
		if e != nil {
			return e
		}
		e = ByteDecoder([]byte(msgStr), &msg)
		if err != nil {
			return e
		}
		return nil
	})
	return
}

// DumpIdx converts message and data of a DHT change request to a string for human consumption
func (ht *BuntHT) dumpIdx(idx int) (str string, err error) {
	var msg Message
	msg, err = ht.GetIdxMessage(idx)
	if err != nil {
		return
	}
	f, _ := msg.Fingerprint()
	str = fmt.Sprintf("MSG (fingerprint %v):\n   %v\n", f, msg)
	switch msg.Type {
	case PUT_REQUEST:
		key := msg.Body.(HoldReq).EntryHash
		entry, entryType, _, _, e := ht.Get(key, StatusDefault, GetMaskAll)
		if e != nil {
			err = fmt.Errorf("couldn't get %v err:%v ", key, e)
			return
		} else {
			str += fmt.Sprintf("DATA: type:%s entry: %v\n", entryType, entry)
		}
	}
	return
}

func statusValueToString(val string) string {
	//TODO
	return val
}

// String converts the table into a human readable string
func (ht *BuntHT) String() (result string) {
	idx, err := ht.GetIdx()
	if err != nil {
		return err.Error()
	}
	result += fmt.Sprintf("DHT changes: %d\n", idx)
	for i := 1; i <= idx; i++ {
		str, err := ht.dumpIdx(i)
		if err != nil {
			result += fmt.Sprintf("%d Error:%v\n", i, err)
		} else {
			result += fmt.Sprintf("%d\n%v\n", i, str)
		}
	}

	result += fmt.Sprintf("DHT entries:\n")
	err = ht.db.View(func(tx *buntdb.Tx) error {
		err = tx.Ascend("entry", func(key, value string) bool {
			x := strings.Split(key, ":")
			k := string(x[1])
			var status string
			statusVal, err := tx.Get("status:" + k)
			if err != nil {
				status = fmt.Sprintf("<err getting status:%v>", err)
			} else {
				status = statusValueToString(statusVal)
			}

			var sources string
			sources, err = tx.Get("src:" + k)
			if err != nil {
				sources = fmt.Sprintf("<err getting sources:%v>", err)
			}
			var links string
			err = tx.Ascend("link", func(key, value string) bool {
				x := strings.Split(key, ":")
				base := x[1]
				link := x[2]
				tag := x[3]
				if base == k {
					links += fmt.Sprintf("Linked to: %s with tag %s\n", link, tag)
					links += value + "\n"
				}
				return true
			})
			result += fmt.Sprintf("Hash--%s (status %s):\nValue: %s\nSources: %s\n%s\n", k, status, value, sources, links)
			return true
		})
		return nil
	})

	return
}

// DumpIdxJSON converts message and data of a DHT change request to a JSON string representation.
func (ht *BuntHT) dumpIdxJSON(idx int) (str string, err error) {
	var msg Message
	var buffer bytes.Buffer
	var msgField, dataField string
	msg, err = ht.GetIdxMessage(idx)

	if err != nil {
		return "", err
	}

	f, _ := msg.Fingerprint()
	buffer.WriteString(fmt.Sprintf("{ \"index\": %d,", idx))
	msgField = fmt.Sprintf("\"message\": { \"fingerprint\": \"%v\", \"content\": \"%v\" },", f, msg)

	switch msg.Type {
	case PUT_REQUEST:
		key := msg.Body.(HoldReq).EntryHash
		entry, entryType, _, _, e := ht.Get(key, StatusAny, GetMaskAll)
		if e != nil {
			err = fmt.Errorf("couldn't get %v err:%v ", key, e)
			return
		}
		dataField = fmt.Sprintf("\"data\": { \"type\": \"%s\", \"entry\": \"%v\" }", entryType, entry)
	}

	if len(dataField) > 0 {
		buffer.WriteString(msgField)
		buffer.WriteString(dataField)
	} else {
		buffer.WriteString(strings.TrimSuffix(msgField, ","))
	}
	buffer.WriteString("}")
	return PrettyPrintJSON(buffer.Bytes())
}

// JSON converts the table into a JSON string representation.
func (ht *BuntHT) JSON() (result string, err error) {
	var buffer, entries bytes.Buffer
	idx, err := ht.GetIdx()
	if err != nil {
		return "", err
	}
	buffer.WriteString("{ \"dht_changes\": [")
	for i := 1; i <= idx; i++ {
		json, err := ht.dumpIdxJSON(i)
		if err != nil {
			return "", fmt.Errorf("DHT Change %d,  Error: %v", i, err)
		}
		buffer.WriteString(json)
		if i < idx {
			buffer.WriteString(",")
		}
	}
	buffer.WriteString("], \"dht_entries\": [")
	err = ht.db.View(func(tx *buntdb.Tx) error {
		err = tx.Ascend("entry", func(key, value string) bool {
			x := strings.Split(key, ":")
			k := string(x[1])
			var status string
			statusVal, err := tx.Get("status:" + k)
			if err != nil {
				status = fmt.Sprintf("<err getting status:%v>", err)
			} else {
				status = statusValueToString(statusVal)
			}

			var sources string
			sources, err = tx.Get("src:" + k)
			if err != nil {
				sources = fmt.Sprintf("<err getting sources:%v>", err)
			}
			var links bytes.Buffer
			err = tx.Ascend("link", func(key, value string) bool {
				x := strings.Split(key, ":")
				base := x[1]
				link := x[2]
				tag := x[3]
				if base == k {
					links.WriteString(fmt.Sprintf("{ \"linkTo\": \"%s\",", link))
					links.WriteString(fmt.Sprintf("\"tag\": \"%s\",", tag))
					links.WriteString(fmt.Sprintf("\"value\": \"%s\" },", EscapeJSONValue(value)))
				}
				return true
			})
			entries.WriteString(fmt.Sprintf("{ \"hash\": \"%s\",", k))
			entries.WriteString(fmt.Sprintf("\"status\": \"%s\",", status))
			entries.WriteString(fmt.Sprintf("\"value\": \"%s\",", EscapeJSONValue(value)))
			entries.WriteString(fmt.Sprintf("\"sources\": \"%s\"", sources))
			if links.Len() > 0 {
				entries.WriteString(fmt.Sprintf(",\"links\": [%s]", strings.TrimSuffix(links.String(), ",")))
			}
			entries.WriteString("},")
			return true
		})
		return nil
	})
	buffer.WriteString(strings.TrimSuffix(entries.String(), ","))
	buffer.WriteString("]}")
	return PrettyPrintJSON(buffer.Bytes())
}

func (ht *BuntHT) Iterate(fn HashTableIterateFn) {
	ht.db.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend("entry", func(key, value string) bool {
			x := strings.Split(key, ":")
			k := string(x[1])
			hash, err := NewHash(k)
			if err != nil {
				return false
			}
			return fn(hash)
		})
		return err
	})
}
