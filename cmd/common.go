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

func GetService(root string) (service *holo.Service, err error) {
	holo.Initialize()
	if root == "" {
		root = os.Getenv("HOLOPATH")
		if root == "" {
			u, err := user.Current()
			if err != nil {
				return nil, err
			}
			userPath := u.HomeDir
			root = userPath + "/" + holo.DefaultDirectoryName
		}
	}
	if initialized := holo.IsInitialized(root); !initialized {
		err = ErrServiceUninitialized
	} else {
		service, err = holo.LoadService(root)
	}
	return
}

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
