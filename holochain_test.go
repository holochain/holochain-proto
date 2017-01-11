package holochain

import (
	"fmt"
	"strconv"
	"testing"
	"time"
	"github.com/google/uuid"
	"os"
	b58 "github.com/jbenet/go-base58"
	"crypto/sha256"
	"crypto/ecdsa"
	gob "encoding/gob"
	"bytes"
	"math/big"
	"github.com/boltdb/bolt"

)

func TestNew(t *testing.T) {
	var key ecdsa.PrivateKey
	h := New("Joe",&key,"some/path")
	nID := string(uuid.NodeID());
	if (nID != string(h.Id.NodeID()) ) {
		t.Error("expected holochain UUID NodeID to be "+nID+" got",h.Id.NodeID())
	}
	if (h.Types[0] != "myData") {
		t.Error("data got:",h.Types)
	}
	if (h.agent != "Joe") {
		t.Error("agent got:",h.agent)
	}
	if (h.privKey != &key) {
		t.Error("key got:",h.privKey)
	}
	if (h.path != "some/path") {
		t.Error("path got:",h.path)
	}

}

func TestGenChain(t *testing.T) {
	err := GenChain()
	ExpectNoErr(t,err)
}

func TestInit(t *testing.T) {
	d := setupTestDir()
	defer cleanupTestDir(d)

	if IsInitialized(d) != false {
		t.Error("expected no directory")
	}
	agent := "Fred Flintstone <fred@flintstone.com>"
	s,err := Init(d, Agent(agent))
	ExpectNoErr(t,err)

	if (string(s.DefaultAgent) != agent) {t.Error("expected "+agent+" got "+string(s.DefaultAgent))}

	ss := fmt.Sprintf("%v",s.Settings)
	if (ss != "{6283 true false}") {t.Error("expected settings {6283 true false} got "+ss)}

	if IsInitialized(d) != true {
		t.Error("expected initialized")
	}
	p := d+"/"+DirectoryName
	privP,err := UnmarshalPrivateKey(p, PrivKeyFileName)
	ExpectNoErr(t,err)

	pub2,err := UnmarshalPublicKey(p,PubKeyFileName)
	ExpectNoErr(t,err)

	if (fmt.Sprintf("%v",*pub2) != fmt.Sprintf("%v",privP.PublicKey)) {t.Error("expected pubkey match!")}

	a,err := readFile(p,AgentFileName)
	ExpectNoErr(t,err)
	if string(a) != agent {t.Error("expected "+agent+" got ",a)}

}


func TestLoadService(t *testing.T) {
	d,service := setupTestService()
	root := service.Path
	defer cleanupTestDir(d)
	s,err := LoadService(root)
	ExpectNoErr(t,err)
	if (s.Path != root) {t.Error("expected path "+d+" got "+s.Path)}
	if (s.Settings.Port != DefaultPort) {t.Error(fmt.Sprintf("expected settings port %d got %d\n",DefaultPort,s.Settings.Port))}
	a := Agent("Herbert <h@bert.com>")
	if (s.DefaultAgent != a) {t.Error("expected agent "+string(a)+" got "+string(s.DefaultAgent))}

}

func TestGenDev(t *testing.T) {
	d,s := setupTestService()
	defer cleanupTestDir(d)
	name := "test"
	root := s.Path+"/"+name
	if err := s.IsConfigured(name); err == nil {
		t.Error("expected no dna got:",err)
	}

	h, err := s.Load("test")
	ExpectErrString(t,err,"open "+root+"/"+DNAFileName+": no such file or directory")

	h,err = GenDev(root)
	if err != nil {
		t.Error("expected no error got",err)
	}

	if err = s.IsConfigured(name); err != nil {
		t.Error(err)
	}

	lh, err := s.Load(name)
	if  err != nil {
		t.Error("Error parsing loading",err)
	}

	if (lh.Id != h.Id) {
		t.Error("expected matching ids!")
	}

	_,err = GenDev(root)
	ExpectErrString(t,err,"holochain: "+root+" already exists")


}

func TestNewEntry(t *testing.T) {
	d,s := setupTestService()
	defer cleanupTestDir(d)
	n := "test"
	path := s.Path+"/"+n
	h,err := GenDev(path)
	ExpectNoErr(t,err)
	myData := `(message (from "art") (to "eric") (contents "test"))`

	hash := b58.Decode("3vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA") // dummy link hash
	var link Hash
	copy(link[:],hash)

	now := time.Unix(1,1) // pick a constant time so the test will always work

	e := GobEntry{C:myData}
	headerHash,header,err := h.NewEntry(now,"myData",link,&e)
	ExpectNoErr(t,err)

	if header.Time != now {t.Error("expected time:"+fmt.Sprintf("%v",now))}

	if header.Type != "myData" {t.Error("expected type myData")}

	// check the header link
	l :=  b58.Encode(header.HeaderLink[:])
	if l != "3vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA" {t.Error("expected header link, got",l)}

	// check the content link
	l =  b58.Encode(header.EntryLink[:])
	if l != "G5tGxuTygAMYx2BMagaWJrYpwtiVuDFUtnYkX6rpL1Y5" {t.Error("expected entry hash, got",l)}

	// check the hash
	// fmt.Printf("HEADER: %v\n",header)
	// hard to check the hash because the signature created each time test runs is
	// different (though valid) so the header will hash to a different value

 	hash = headerHash[:]
	a := b58.Encode(hash)

	b,err := ByteEncoder(&header)
	hh := Hash(sha256.Sum256(b))
	if (a != b58.Encode(hh[:])) {
		t.Error("expected header hash match!")
	}

	/*	if a != "EdkgsdwazMZc9vJJgGXgbGwZFvy2Wa1hLCjngmkw3PbF" {
		t.Error("expected EdkgsdwazMZc9vJJgGXgbGwZFvy2Wa1hLCjngmkw3PbF got:",a)
	}*/

	// check the my signature of the entry
	pub,err := UnmarshalPublicKey(s.Path,PubKeyFileName)
	ExpectNoErr(t,err)
	sig := header.MySignature
	hash = header.EntryLink[:]
	if !ecdsa.Verify(pub,hash,sig.R,sig.S) {t.Error("expected verify!")}

	err = h.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Headers"))
		v := b.Get(headerHash[:])

		s1 := fmt.Sprintf("%v",*header)

		var h2 Header
		err := ByteDecoder(v,&h2)

		if err != nil {return err}
		s2 := fmt.Sprintf("%v",h2)
		if s2!=s1 {t.Error("expected header to match! \n  "+s2+" \n  "+s1)}

		b = tx.Bucket([]byte("Entries"))
		v = b.Get(header.EntryLink[:])
		s1 = fmt.Sprintf("%v",myData)

		var d2 GobEntry
		err = d2.Unmarshal(v)

		if err != nil {return err}
		s2 = fmt.Sprintf("%v",d2.C)
		if s2!=s1 {t.Error("expected entry to match! \n  "+s2+" \n  "+s1)}


		return nil
	})
	ExpectNoErr(t,err)

}

func TestHeader(t *testing.T) {
	var h1,h2 Header
	h1 = mkTestHeader()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&h1)
	ExpectNoErr(t,err)

	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&h2)

	s1 := fmt.Sprintf("%v",h1)
	s2 := fmt.Sprintf("%v",h2)
	if s2!=s1 {t.Error("expected binary match! "+s2+" "+s1)}
}

func TestGob(t *testing.T) {
	gob.Register(Header{})

	g := GobEntry{C:mkTestHeader()}
	v,err := g.Marshal()
	ExpectNoErr(t,err)
	var g2 GobEntry
	err = g2.Unmarshal(v)
	ExpectNoErr(t,err)
	sg1 := fmt.Sprintf("%v",g)
	sg2 := fmt.Sprintf("%v",g)
	if sg2!=sg1 {t.Error("expected gob match! \n  "+sg2+" \n  "+sg1)}
}

func TestJSONEntry(t *testing.T) {
	/* Not yet implemented or used
	g := JSONEntry{C:Config{Port:8888}}
	v,err := g.Marshal()
	ExpectNoErr(t,err)
	var g2 JSONEntry
	err = g2.Unmarshal(v)
	ExpectNoErr(t,err)
	if g2!=g {t.Error("expected JSON match! "+fmt.Sprintf("%v",g)+" "+fmt.Sprintf("%v",g2))}
*/
}

//----------------------------------------------------------------------------------------

func ExpectErrString(t *testing.T,err error,text string) {
	if err.Error() != text {
		t.Error("expected '"+text+"' got",err)
	}
}

func ExpectNoErr(t *testing.T,err error) {
	if err != nil {
		t.Error("expected no err, got",err)
	}
}

func mkTestDirName() string {
	t := time.Now()
	d := "/tmp/holochain_test"+strconv.FormatInt(t.Unix(),10)+"."+strconv.Itoa(t.Nanosecond())
	return d
}

func setupTestService() (d string,s *Service) {
	d = mkTestDirName()
	agent := Agent("Herbert <h@bert.com>")
	s,err := Init(d,agent)
	if err != nil {panic(err)}
	return
}

func setupTestDir() string {
	d := mkTestDirName();
	return d
}

func cleanupTestDir(path string) {
	func() {os.RemoveAll(path)}()
}

func mkTestHeader() Header {
	var hl,el Hash
	copy(hl[:],b58.Decode("1vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA"))
	copy(el[:],b58.Decode("2vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA"))
	now := time.Unix(1,1) // pick a constant time so the test will always work
	h1 := Header{Time:now,Type:"fish",Meta:"dog",
		HeaderLink:hl,
		EntryLink:el,
		MySignature:Signature{R:new(big.Int),S:new(big.Int)},
	}
	h1.MySignature.R.SetUint64(123)
	h1.MySignature.S.SetUint64(321)
	return h1
}
