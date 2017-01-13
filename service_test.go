package holochain

import (
	"fmt"
	"testing"
)


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
