// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// non exported utility functions for Holochain package

package holochain
import (
	"os"
	"errors"
	"github.com/BurntSushi/toml"
	"io/ioutil"

)

func writeToml(path string,file string,data interface{},overwrite bool) error {
	p := path+"/"+file
	if !overwrite && fileExists(p) {
		return mkErr(path+" already exists")
	}
	f, err := os.Create(p)
	if err != nil {return err}

	defer f.Close()
	enc := toml.NewEncoder(f)
	err = enc.Encode(data);
	return err
}

func writeFile(path string,file string,data []byte) error {
	p := path+"/"+file
	if fileExists(p) {return mkErr(path+" already exists")}
	f, err := os.Create(p)
	if err != nil {return err}
	defer f.Close()

	l,err := f.Write(data)
	if (err != nil) {return err}

	if (l != len(data)) {return mkErr("unable to write all data")}
	f.Sync()
	return err
}

func readFile(path string,file string) (data []byte, err error) {
	p := path+"/"+file
	data, err = ioutil.ReadFile(p)
	return data,err
}

func mkErr(err string) error {
	return errors.New("holochain: "+err)
}

func dirExists(name string) bool {
	info, err := os.Stat(name)
	return err == nil &&  info.Mode().IsDir();
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {return false}
	return info.Mode().IsRegular();
}
