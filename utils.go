// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// non exported utility functions for Holochain package

package holochain

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/ghodss/yaml"
	"github.com/lestrrat/go-jsschema"
	"github.com/lestrrat/go-jsval/builder"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	//	"sync"
	"time"
)

const (
	OS_READ        = 04
	OS_WRITE       = 02
	OS_EX          = 01
	OS_USER_SHIFT  = 6
	OS_GROUP_SHIFT = 3
	OS_OTH_SHIFT   = 0

	OS_USER_R   = OS_READ << OS_USER_SHIFT
	OS_USER_W   = OS_WRITE << OS_USER_SHIFT
	OS_USER_X   = OS_EX << OS_USER_SHIFT
	OS_USER_RW  = OS_USER_R | OS_USER_W
	OS_USER_RWX = OS_USER_RW | OS_USER_X

	OS_GROUP_R   = OS_READ << OS_GROUP_SHIFT
	OS_GROUP_W   = OS_WRITE << OS_GROUP_SHIFT
	OS_GROUP_X   = OS_EX << OS_GROUP_SHIFT
	OS_GROUP_RW  = OS_GROUP_R | OS_GROUP_W
	OS_GROUP_RWX = OS_GROUP_RW | OS_GROUP_X

	OS_OTH_R   = OS_READ << OS_OTH_SHIFT
	OS_OTH_W   = OS_WRITE << OS_OTH_SHIFT
	OS_OTH_X   = OS_EX << OS_OTH_SHIFT
	OS_OTH_RW  = OS_OTH_R | OS_OTH_W
	OS_OTH_RWX = OS_OTH_RW | OS_OTH_X

	OS_ALL_R   = OS_USER_R | OS_GROUP_R | OS_OTH_R
	OS_ALL_W   = OS_USER_W | OS_GROUP_W | OS_OTH_W
	OS_ALL_X   = OS_USER_X | OS_GROUP_X | OS_OTH_X
	OS_ALL_RW  = OS_ALL_R | OS_ALL_W
	OS_ALL_RWX = OS_ALL_RW | OS_GROUP_X
)

func writeToml(path string, file string, data interface{}, overwrite bool) error {
	p := filepath.Join(path, file)
	if !overwrite && FileExists(p) {
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

func WriteFile(data []byte, pathParts ...string) error {
	p := filepath.Join(pathParts...)
	if FileExists(p) {
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

func ReadFile(pathParts ...string) (data []byte, err error) {
	p := filepath.Join(pathParts...)
	data, err = ioutil.ReadFile(p)
	return data, err
}

func mkErr(err string) error {
	return errors.New("holochain: " + err)
}

func DirExists(pathParts ...string) bool {
	path := filepath.Join(pathParts...)
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsDir()
}

func FileExists(pathParts ...string) bool {
	path := filepath.Join(pathParts...)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

func filePerms(pathParts ...string) (perms os.FileMode, err error) {
	var fi os.FileInfo
	fi, err = os.Stat(filepath.Join(pathParts...))
	if err != nil {
		return
	}
	perms = fi.Mode().Perm()
	return
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
		return fmt.Errorf("Destination (%s) already exists", dest)
	}

	// create dest dir

	err = os.MkdirAll(dest, fi.Mode())
	if err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(source)

	for _, entry := range entries {

		sfp := filepath.Join(source, entry.Name())
		dfp := filepath.Join(dest, entry.Name())
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

// Encode encodes data to the writer according to the given format
func Encode(writer io.Writer, format string, data interface{}) (err error) {
	switch format {
	case "toml":
		enc := toml.NewEncoder(writer)
		err = enc.Encode(data)

	case "json":
		enc := json.NewEncoder(writer)
		enc.SetIndent("", "    ")
		err = enc.Encode(data)

	case "yml":
		fallthrough
	case "yaml":
		y, e := yaml.Marshal(data)
		if e != nil {
			err = e
			return
		}
		n, e := writer.Write(y)
		if e != nil {
			err = e
			return
		}
		if n != len(y) {
			err = errors.New("unable to write all bytes while encoding")
		}

	default:
		err = errors.New("unknown encoding format: " + format)
	}
	return
}

// Decode extracts data from the reader according to the type
func Decode(reader io.Reader, format string, data interface{}) (err error) {
	switch format {
	case "toml":
		_, err = toml.DecodeReader(reader, data)
	case "json":
		dec := json.NewDecoder(reader)
		err = dec.Decode(data)
	case "yml":
		fallthrough
	case "yaml":
		y, e := ioutil.ReadAll(reader)
		if e != nil {
			err = e
			return
		}
		err = yaml.Unmarshal(y, data)
	default:
		err = errors.New("unknown encoding format: " + format)
	}
	return
}

// EncodingFormat returns the files format if supported otherwise ""
func EncodingFormat(file string) (f string) {
	s := strings.Split(file, ".")
	f = s[len(s)-1]
	if f == "json" || f == "yml" || f == "yaml" || f == "toml" {
		return
	}
	f = ""
	return
}

// ByteEncoder encodes anything using gob
func ByteEncoder(data interface{}) (b []byte, err error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(data)
	if err != nil {
		return
	}
	b = buf.Bytes()
	return
}

// ByteDecoder decodes data encoded by ByteEncoder
func ByteDecoder(b []byte, to interface{}) (err error) {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(to)
	return
}

func BuildJSONSchemaValidatorFromFile(path string) (validator *JSONSchemaValidator, err error) {
	var s *schema.Schema
	s, err = schema.ReadFile(path)
	if err != nil {
		return
	}

	b := builder.New()
	var v JSONSchemaValidator
	v.v, err = b.Build(s)
	if err == nil {
		validator = &v
	}
	return
}

func BuildJSONSchemaValidatorFromString(input string) (validator *JSONSchemaValidator, err error) {
	var s *schema.Schema
	s, err = schema.Read(strings.NewReader(input))
	if err != nil {
		return
	}
	b := builder.New()
	var v JSONSchemaValidator
	v.v, err = b.Build(s)
	if err == nil {
		validator = &v
	}
	return
}

// Ticker runs a function on an interval that can be stopped with the returned bool channel
func Ticker(interval time.Duration, fn func()) (stopper chan bool) {
	ticker := time.NewTicker(interval)
	stopper = make(chan bool, 1)
	go func() {
		//	var lk sync.RWMutex
		var stopped bool
		for {
			select {
			case <-ticker.C:
				//		lk.RLock()
				if !stopped {
					fn()
				}
				//		lk.Unlock()
			case <-stopper:
				//		lk.Lock()
				stopped = true
				//		lk.Unlock()
				return
			}
		}
	}()
	return
}
