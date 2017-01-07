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
	if (h.LinkEncoding != "JSON") {
		t.Error("expected default encoding to be JSON, got:",h.LinkEncoding)
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
	err = Init(d)
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

	ExpectNoErr(t,err)

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

		if IsConfigured(d) != false {
			t.Error("expected no config")
		}

		_, err := Load(d)
		ExpectErrString(t,err,"holochain: missing dna.conf")

		h,err := GenDev(d)
		if err != nil {
			t.Error("expected no error got",err)
		}

		if IsConfigured(d) != true {
			t.Error("expected config")
		}


		lh, err := Load(d)
		if  err != nil {
			t.Error("Error parsing loading",err)
		}
		if (lh.LinkEncoding != "JSON") {
			t.Error("expected default encoding to be JSON, got:",lh.LinkEncoding)
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
