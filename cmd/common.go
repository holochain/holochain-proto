// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// utilities for the holochain commands

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

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
