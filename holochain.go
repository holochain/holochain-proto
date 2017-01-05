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
	LinkEncoding string
}

func IsInitialized() bool {
	_, err := os.Stat(DirectoryName)
	return err == nil;
}

// New creates a new holochain structure with a randomly generated ID and default values
func New() Holochain {
	u,err := uuid.NewUUID()
	if err != nil {panic(err)}
	return Holochain {Id:u,LinkEncoding:"JSON"}
}

// Load creates a holochain structure from the configuration files
func Load() (h Holochain,err error) {
	if IsInitialized() {
		_,err = toml.DecodeFile(ConfigPath, &h)
	} else {
		err = errors.New("holochain: missing .holochain directory")
	}
	return h,err
}

// Init setts up a holochain by creating the initial genesis links.
// It assumes a properly set up .holochain sub-directory with a config file and
// keys for signing.  See Gen
func Init() error {

	return nil
}

// Gen initializes the current directory with a template config file suitable for editing.
func Gen() (hP *Holochain, err error) {
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
			//var config Config
			//config.H = h
			if err := enc.Encode(h); err == nil {
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
