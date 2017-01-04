package holochain

import (
//	"fmt"
	"testing"
	"github.com/google/uuid"
	"os"
	"github.com/BurntSushi/toml"
)

func TestNew(t *testing.T) {
	h := New()
	nID := string(uuid.NodeID());
	if (nID != string(h.Id.NodeID()) ) {
		t.Error("expected holocain UUID NodeID to be "+nID+" got",h.Id.NodeID())
	}
}

func TestInit(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {panic(err)}
	defer func() {os.Chdir(pwd)}()
	d := "/tmp/holochain_test";
	if err := os.Mkdir(d,os.ModePerm); err == nil {
		defer func() {os.RemoveAll(d)}()
		if err := os.Chdir(d); err != nil {
			panic(err)
		}

		if IsInitialized() != false {
			t.Error("expected false")
		}

		_,err := Init()
		if err != nil {
			t.Error("expected no error got",err)
		}

		if IsInitialized() != true {
			t.Error("expected true")
		}

		var config Config
		if _, err := toml.DecodeFile(d+"/"+ConfigPath, &config); err != nil {
			t.Error("Error parsing config file")
		}
		_,err = Init()
		if err.Error() != "holochain: already initialized" {
			t.Error("expected 'holochain: already initialized' got",err)
		}
	}
}
