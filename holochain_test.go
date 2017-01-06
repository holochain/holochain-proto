package holochain

import (
	"strconv"
	"testing"
	"time"
	"github.com/google/uuid"
	"os"
//	"crypto/ecdsa"

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

	if IsInitialized(d) != false {
		t.Error("expected no directory")
	}
	err = Init(d)
	ExpectNoErr(t,err)

	if IsInitialized(d) != true {
		t.Error("expected directory")
	}
/*
	var priv ecdsa.PrivateKey
	_,err = toml.DecodeFile(d+"/"+PrivKeyFileName, &priv)
	ExpectNoErr(t,err)

	var pub ecdsa.PublicKey
	_,err = toml.DecodeFile(d+"/"+PubKeyFileName, &pub)
	ExpectNoErr(t,err)

	if priv.Public() != pub {t.Error("pub key doesn't match!")} */
}

func TestGenDev(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {panic(err)}
	defer func() {os.Chdir(pwd)}()
	d := mkTestDir()
	if err := os.Mkdir(d,os.ModePerm); err == nil {
		defer func() {os.RemoveAll(d)}()
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
		ExpectErrString(t,err,"holochain: dna.conf already exists")

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
	return "/tmp/holochain_test"+strconv.FormatInt(time.Now().Unix(),10);
}
