// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// utilities for the holochain commands

package cmd

import (
	"errors"
	"fmt"
	"net"
	"os"
	exec "os/exec"
	"os/user"
	"path/filepath"
	"syscall"

	holo "github.com/metacurrency/holochain"
)

var debug bool = false

var ErrServiceUninitialized = errors.New("service not initialized, run 'hcdev init'")

func GetCurrentDirectory() (dir string, err error) {
	dir, err = os.Getwd()
	return
}

func syscallExec(binaryFile string, args ...string) error {
	return syscall.Exec(binaryFile, append([]string{binaryFile}, args...), os.Environ())
}

func ExecBinScript(script string, args ...string) (err error) {
	var path string
	path, err = GolangHolochainDir("bin", script)
	if err != nil {
		return
	}
	if debug {
		fmt.Printf("HC: common.go: ExecBinScript: %v (%v)", path, args)
	}
	return syscallExec(path, args...)
}

func OsExecSilent(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	if debug {
		fmt.Printf("common.go: OsExecSilent: %v", cmd)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	if debug {
		fmt.Printf("HC: common.go: OsExecSilent: %v", output)
	}

	return nil
}

// OsExecPipes executes a command as if we are in a shell, including user input
func OsExecPipes(args ...string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	if debug {
		fmt.Printf("HC: common.go: OsExecSilent: %v", cmd)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	cmd.Run()

	return cmd
}

// OsExecPipes executes a command as if we are in a shell, including user input
func OsExecPipes_noRun(args ...string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	if debug {
		fmt.Printf("HC: common.go: OsExecPipes_noRun: %v", cmd)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd
}

// IsAppDir tests path to see if it's a properly set up holochain app
// returns nil on success or error describing the problem
func IsAppDir(path string) error {
	// return fmt.Errorf("this isnt of any use at the moment")

	info, err := os.Stat(filepath.Join(path, "dna", "dna.json"))
	if err != nil {
		err = fmt.Errorf("directory missing dna/dna.json file")
	} else {
		if !info.Mode().IsRegular() {
			err = fmt.Errorf("dna/dna.json is not a file")
		}
	}
	return err
}

// IsCoreDir tests path to see if it is contains Holochain Core source files
// returns nil on success or an error describing the problem
// func IsCoreDir(path string) error {
// check for the existance of package.json
//
// return IsFile(filepath.Join(path, "package.json")
// }

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

func Die(message string) {
	fmt.Println(message)
	os.Exit(1)
}

func GolangHolochainDir(subPath ...string) (path string, err error) {
	err = nil
	joinable := append([]string{os.Getenv("GOPATH"), "src/github.com/metacurrency/holochain"}, subPath...)
	path = filepath.Join(joinable...)
	return
}

func IsFile(path ...string) bool {
	return IsFileFromString(filepath.Join(path...))
}
func IsFileFromString(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	} else {
		if !info.Mode().IsRegular() {
			return false
		}
	}

	return true
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

func MakeTmpDir(name string) (tmpHolochainCopyDir string, err error) {
	tmpHolochainCopyDir = filepath.Join("/", "tmp", name)
	os.RemoveAll(tmpHolochainCopyDir)
	err = os.MkdirAll(tmpHolochainCopyDir, 0770)
	if err != nil {
		return "", err
	}
	return
}

// Ask the kernel for a free open port that is ready to use
func GetFreePort() (port int, err error) {
	port = -1
	err = nil

	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return
	}
	defer l.Close()
	port = l.Addr().(*net.TCPAddr).Port
	return
}


// var syncWatcher fsnotify.Watcher
// var created_syncWatcher bool

// func SyncStart(syncName string) err error {
//   err = nil

//   syncDir := filepath.Join("/tmp", "hc.sync")
//   if !DirExists(syncDir) {
//     err = os.MkdirAll(syncDir, "0666")
//     if err != nil {
//       return err
//     }
//   }

//   syncFile := filepath.Join(syncDir, syncName)
//   if FileExists(syncPath) {
//     return errors.New("HC: common.go: SyncStart(%v): file already exists", syncFile)
//   }

//   os.OpenFile(syncFile, os.O_RDONLY|os.O_CREATE, 0666)
// }

// func SyncOnRM(syncName string) (err error) {
//   syncFile := filepath.Join("/tmp", "hc.sync", syncName)
//   if !FileExists(syncFile) {
//     return errors.New("HC: common.go: SyncOnRM(%v)", syncFile)
//   }

//   if ! created_syncWatcher {
//     watcher, err := fsnotify.NewWatcher()
//     if err != nil {
//       log.Fatal(err)
//     }
//     defer watcher.Close()

//     done := make(chan bool)
//     go func() {
//       for {
//         select {
//         case event := <-watcher.Events:
//           log.Println("event:", event)
//           if event.Op&fsnotify.Remove == fsnotify.Remove {
//             log.Println("modified file:", event.Name)
//           }
//         case err := <-watcher.Errors:
//           log.Println("error:", err)
//         }
//       }
//     }()
//     created_syncWatcher = true
//   }

//   err = watcher.Add(syncPath)
//   if err != nil {
//     return err
//   }
//   <-done
// }

// func 

