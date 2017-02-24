// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// non exported utility functions for Holochain package

package holochain

import (
	"errors"
	"github.com/BurntSushi/toml"
	"io"
	"io/ioutil"
	"os"
)

func writeToml(path string, file string, data interface{}, overwrite bool) error {
	p := path + "/" + file
	if !overwrite && fileExists(p) {
		return mkErr(path + " already exists")
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}

	defer f.Close()
	enc := toml.NewEncoder(f)
	err = enc.Encode(data)
	return err
}

func writeFile(path string, file string, data []byte) error {
	p := path + "/" + file
	if fileExists(p) {
		return mkErr(p + " already exists")
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	l, err := f.Write(data)
	if err != nil {
		return err
	}

	if l != len(data) {
		return mkErr("unable to write all data")
	}
	f.Sync()
	return err
}

func readFile(path string, file string) (data []byte, err error) {
	p := path + "/" + file
	data, err = ioutil.ReadFile(p)
	return data, err
}

func mkErr(err string) error {
	return errors.New("holochain: " + err)
}

func dirExists(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.Mode().IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
func CopyDir(source string, dest string) (err error) {

	// get properties of source dir
	fi, err := os.Stat(source)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return errors.New("Source is not a directory")
	}

	// ensure dest dir does not already exist

	_, err = os.Open(dest)
	if !os.IsNotExist(err) {
		return errors.New("Destination already exists")
	}

	// create dest dir

	err = os.MkdirAll(dest, fi.Mode())
	if err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(source)

	for _, entry := range entries {

		sfp := source + "/" + entry.Name()
		dfp := dest + "/" + entry.Name()
		if entry.IsDir() {
			err = CopyDir(sfp, dfp)
			if err != nil {
				return err
			}
		} else {
			// perform copy
			err = CopyFile(sfp, dfp)
			if err != nil {
				return err
			}
		}

	}
	return
}

// CopyFile copies file source to destination dest.
func CopyFile(source string, dest string) (err error) {
	sf, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sf.Close()
	df, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer df.Close()
	_, err = io.Copy(df, sf)
	if err == nil {
		var si os.FileInfo
		si, err = os.Stat(source)
		if err == nil {
			err = os.Chmod(dest, si.Mode())
		}
	}
	return
}
