// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// utilities for the holochain commands

package cmd

import (
	"errors"
	"fmt"
	holo "github.com/metacurrency/holochain"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
)

var ErrServiceUninitialized = errors.New("service not initialized, run 'hcdev init'")

func GetCurrentDirectory() (dir string, err error) {
	dir, err = os.Getwd()
	return
}

func syscallExec(binaryFile string, args ...string) error {
	return syscall.Exec(binaryFile, append([]string{binaryFile}, args...), os.Environ())
}

func ExecBinScript(script string, args ...string) error {
	path := filepath.Join(
		os.Getenv("GOPATH"),
		"src/github.com/metacurrency/holochain",
		"bin",
		script)
	return syscallExec(path, args...)
}

// IsAppDir tests path to see if it's a properly set up holochain app
// returns nil on success or error describing the problem
func IsAppDir(path string) error {
	info, err := os.Stat(filepath.Join(path, ".hc"))
	if err != nil {
		err = fmt.Errorf("directory missing .hc subdirectory")
	} else {
		if !info.Mode().IsDir() {
			err = fmt.Errorf(".hc is not a directory")
		}
	}
	return err
}

// GetService is a helper function to load the holochain service from default locations or a given path
func GetService(root string) (service *holo.Service, err error) {
	holo.InitializeHolochain()
	if root == "" {
		root = os.Getenv("HOLOPATH")
		if root == "" {
			u, err := user.Current()
			if err != nil {
				return nil, err
			}
			userPath := u.HomeDir
			root = filepath.Join(userPath, holo.DefaultDirectoryName)
		}
	}
	if initialized := holo.IsInitialized(root); !initialized {
		err = ErrServiceUninitialized
	} else {
		service, err = holo.LoadService(root)
	}
	return
}

// GetHolochain os a helper function to load a holochain from a directory or report an error based on a command name
func GetHolochain(name string, service *holo.Service, cmd string) (h *holo.Holochain, err error) {
	if service == nil {
		err = ErrServiceUninitialized
		return
	}

	if name == "" {
		err = errors.New("missing required holochain-name argument to " + cmd)
		return
	}

	h, err = service.Load(name)
	if err != nil {
		return
	}
	return
}

//MakeDirs creates the directory structure of an application
func MakeDirs(devPath string) error {
	err := os.MkdirAll(devPath, os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(devPath, holo.ChainDNADir), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(devPath, holo.ChainUIDir), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(devPath, holo.ChainTestDir), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
