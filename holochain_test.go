package holochain

import (
	"fmt"
	"strconv"
	"testing"
	"time"
	"github.com/google/uuid"
	"os"
	b58 "github.com/jbenet/go-base58"
	"crypto/ecdsa"
)

func TestNew(t *testing.T) {
	h := New()
	nID := string(uuid.NodeID());
	if (nID != string(h.Id.NodeID()) ) {
		t.Error("expected holocain UUID NodeID to be "+nID+" got",h.Id.NodeID())
	}
	if (h.Types[0] != "myData") {
		t.Error("data got:",h.Types)
	}
}

func TestGenChain(t *testing.T) {
	err := GenChain()
	ExpectNoErr(t,err)
}

func TestInit(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {panic(err)}
	defer func() {os.Chdir(pwd)}()
	d := mkTestDir();
	defer func() {os.RemoveAll(d)}()

	if IsInitialized(d) != false {
		t.Error("expected no directory")
	}
	agent := "Fred Flintstone <fred@flintstone.com>"
	err = Init(d, Agent(agent))
	ExpectNoErr(t,err)

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

func TestGenDev(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {panic(err)}
	defer func() {os.Chdir(pwd)}()
	d := mkTestDir()
	defer func() {os.RemoveAll(d)}()
	if err := os.Mkdir(d,os.ModePerm); err == nil {
		if err := os.Chdir(d); err != nil {
			panic(err)
		}

		if err = IsConfigured(d); err == nil {
			t.Error("expected no dna got:",err)
		}

		_, err := Load(d)
		ExpectErrString(t,err,"open "+d+"/"+DNAFileName+": no such file or directory")

		h,err := GenDev(d)
		if err != nil {
			t.Error("expected no error got",err)
		}

		if err = IsConfigured(d); err != nil {
			t.Error(err)
		}

		lh, err := Load(d)
		if  err != nil {
			t.Error("Error parsing loading",err)
		}

		if (lh.Id != h.Id) {
			t.Error("expected matching ids!")
		}

		_,err = GenDev(d)
		ExpectErrString(t,err,"holochain: "+d+" already exists")

	}
}

func TestNewEntry(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {panic(err)}
	defer func() {os.Chdir(pwd)}()
	d := mkTestDir();
	defer func() {os.RemoveAll(d)}()
	agent := Agent("Herbert <h@bert.com>")
	err = Init(d,agent)
	ExpectNoErr(t,err)
	root := d+"/"+DirectoryName
	n := "test"
	h,err := GenDev(root+"/"+n)
	ExpectNoErr(t,err)
	userContent := `{
"from": "Art"
"msg": "Hi there!"
}
`
	hash := b58.Decode("3vemK25pc5ewYtztPGYAdX39uXuyV13xdouCnZUr8RMA") // dummy link hash
	var link EntryHash
	copy(link[:],hash)
	links := []PartyLink {{Party:agent, Link:link}}

	agent,key,err := LoadSigner(root)
	ExpectNoErr(t,err)

	e,err := NewEntry(agent,key,h,EntryType("JSON"),links,userContent)
	ExpectNoErr(t,err)

	// check the content
	if e.Data.Type != EntryType("JSON") {t.Error("expected type JSON")}
	if e.Data.UserContent != userContent {t.Error("expected entry to have user content")}
	s := fmt.Sprintf("%v",e.Data.Links)
	if s != "[{Herbert <h@bert.com> [43 117 219 74 12 22 62 60 12 80 253 26 109 133 191 237 6 212 179 213 59 161 252 238 239 96 149 93 100 245 26 121]}]" {t.Error("expected links got ",s)}

	// check the adddress
	hash = e.Address[:]
	a := b58.Encode(hash)
	if a != "CbKR5iA6ptgdsDLysiZ3R2ZMebqxUB5fGJCQaXQRFgDc" {
		t.Error("expected CbKR5iA6ptgdsDLysiZ3R2ZMebqxUB5fGJCQaXQRFgDc got:",a)
	}

	// check the signature
	if e.Signatures[0].Signer != agent {t.Error("expected agent got",e.Signatures[0].Signer)}
	pub,err := UnmarshalPublicKey(root,PubKeyFileName)
	ExpectNoErr(t,err)
	sig := e.Signatures[0].Sig
	if !ecdsa.Verify(pub,hash,&sig.R,&sig.S) {t.Error("expected verify!")}
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

func mkTestDir() string {
	t := time.Now()
	d := "/tmp/holochain_test"+strconv.FormatInt(t.Unix(),10)+"."+strconv.Itoa(t.Nanosecond())
	return d
}
