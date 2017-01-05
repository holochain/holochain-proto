package holochain

import (
//	"fmt"
	"testing"
	"time"
	"github.com/google/uuid"
	"os"

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

func TestInit(t *testing.T) {
	Init()
	t.Error("not implmented")
}

func TestGen(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {panic(err)}
	defer func() {os.Chdir(pwd)}()
	d := "/tmp/holochain_test"+string(time.Now().Unix());
	if err := os.Mkdir(d,os.ModePerm); err == nil {
		defer func() {os.RemoveAll(d)}()
		if err := os.Chdir(d); err != nil {
			panic(err)
		}

		if IsInitialized() != false {
			t.Error("expected false")
		}

		_, err := Load()
		if  err.Error() != "holochain: missing .holochain directory" {
			t.Error("expected 'holochain: missing .holochain directory' got",err)
		}

		h,err := Gen()
		if err != nil {
			t.Error("expected no error got",err)
		}

		if IsInitialized() != true {
			t.Error("expected true")
		}

		lh, err := Load()
		if  err != nil {
			t.Error("Error parsing loading",err)
		}
		if (lh.LinkEncoding != "JSON") {
			t.Error("expected default encoding to be JSON, got:",lh.LinkEncoding)
		}

		if (lh.Id != h.Id) {
			t.Error("expected matching ids!")
		}

		_,err = Gen()
		if err.Error() != "holochain: already initialized" {
			t.Error("expected 'holochain: already initialized' got",err)
		}
	}
}
