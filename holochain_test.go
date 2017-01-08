package holochain

import (
	"fmt"
	"strconv"
	"testing"
	"time"
	"github.com/google/uuid"
	"os"

//	"github.com/BurntSushi/toml"


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
	err = Init(d, agent)
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
