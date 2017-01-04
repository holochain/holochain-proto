package holochain

import (
	"os"
	"errors"
	"github.com/google/uuid"
	"github.com/BurntSushi/toml"
)

const Version string = "0.1.1"

const (
	DirectoryName string = ".holochain"
	ConfigFileName string = "config.toml"
	ConfigPath string = DirectoryName+"/"+ConfigFileName
)

type Holochain struct {
	Id uuid.UUID
}

type Config struct {
	H Holochain
}

func IsInitialized() bool {
	_, err := os.Stat(DirectoryName)
	return err == nil;
}

func New() Holochain {
	u,err := uuid.NewUUID()
	if err != nil {panic(err)}
	return Holochain {Id:u}
}

func Init() (hP *Holochain, err error) {
	var h Holochain
	if !IsInitialized() {
		if err := os.Mkdir(DirectoryName,os.ModePerm); err == nil {
			h = New()

			f, err := os.Create(ConfigPath)
			if err != nil {
				return nil,err
			}
			defer f.Close()

			enc := toml.NewEncoder(f)
			var config Config
			config.H = h
			if err := enc.Encode(config); err == nil {
				hP = &h
			}
		}
	} else {
		err = errors.New("holochain: already initialized")
	}

	return
}
/*func Link(h *Holochain, data interface{}) error {

}*/
