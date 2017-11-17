// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// utilities for the holochain commands

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/urfave/cli"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"syscall"
	"time"

	holo "github.com/metacurrency/holochain"
)

var ErrServiceUninitialized = errors.New("service not initialized, run 'hcadmin init'")

func MakeErr(c *cli.Context, text string) error {
	if c != nil {
		text = fmt.Sprintf("%s: %s", c.Command.Name, text)
	}

	if os.Getenv("HC_TESTING") != "" {
		os.Setenv("HC_TESTING_EXITERR", fmt.Sprintf("%d", 1))
		fmt.Printf(text)
		return errors.New(text)
	} else {
		return cli.NewExitError(text, 1)
	}
}

func MakeErrFromErr(c *cli.Context, err error) error {
	return MakeErr(c, err.Error())
}

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

	holo.Debugf("ExecBinScript: %v (%v)", path, args)

	return syscallExec(path, args...)
}

func OsExecSilent(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	holo.Debugf("OsExecSilent: %v", cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	holo.Debugf("OsExecSilent: %v", output)

	return nil
}

// OsExecPipes executes a command as if we are in a shell, including user input
func OsExecPipes(args ...string) *exec.Cmd {
	cmd := OsExecPipes_noRun(args...)
	holo.Debugf("OsExecPipes: %v", cmd)
	cmd.Run()
	return cmd
}

// OsExecPipes executes a command as if we are in a shell, including user input
func OsExecPipes_noRun(args ...string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	holo.Debugf("OsExecPipes_noRun: %v", cmd)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// cmd.Stdin = os.Stdin

	return cmd
}

// RunAppWithStdoutCapture runs a cli.App and captures the stdout
func RunAppWithStdoutCapture(app *cli.App, args []string, wait time.Duration) (out string, err error) {
	os.Args = args

	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() { err = app.Run(os.Args) }()

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	time.Sleep(wait)

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	out = <-outC
	return
}

var configExtensionList []string

func GetConfigExtensionList() (conExtList []string) {
	if configExtensionList == nil {
		configExtensionList = []string{"json", "toml", "yaml", "yml"}
	}
	return configExtensionList
}

// IsAppDir tests path to see if it's a properly set up holochain app
// returns nil on success or error describing the problem
func IsAppDir(path string) (err error) {
	err = nil

	for _, filename := range GetConfigExtensionList() {
		info, err := os.Stat(filepath.Join(path, "dna", "dna."+filename))
		if err != nil {
			// err = fmt.Errorf("directory missing dna/%v file", filename)
		} else {
			if info.Mode().IsRegular() {
				return nil
			}
		}
	}
	err = fmt.Errorf("HC: Holochain App directory missing dna/dna.xyz config file")
	return err
}

// IsCoreDir tests path to see if it is contains Holochain Core source files
// returns nil on success or an error describing the problem
// func IsCoreDir(path string) error {
// check for the existance of package.json
//
// return IsFile(filepath.Join(path, "package.json")
// }

// GetHolochainRoot returns either the path from the environment variable or the default
func GetHolochainRoot(root string) (string, error) {
	if root == "" {
		root = os.Getenv("HOLOPATH")
		if root == "" {
			u, err := user.Current()
			if err != nil {
				return "", err
			}
			userPath := u.HomeDir
			root = filepath.Join(userPath, holo.DefaultDirectoryName)
		}
	}
	return root, nil
}

// GetService is a helper function to load the holochain service from default locations or a given path
func GetService(root string) (service *holo.Service, err error) {
	holo.InitializeHolochain()
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

	val := os.Getenv("HOLOCHAINCONFIG_ENABLENATUPNP")
	if val != "" {
		h.Config.EnableNATUPnP = val == "true"
	}

	if err = h.Prepare(); err != nil {
		return
	}
	return
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

func IsDir(pathParts ...string) bool {
	path := filepath.Join(pathParts...)
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsDir()
}

func GetTmpDir(name string) (d string, err error) {
	d = filepath.Join("/", "tmp", name)
	//d, err = ioutil.TempDir("", d)
	return
}

func MakeTmpDir(name string) (tmpHolochainCopyDir string, err error) {
	tmpHolochainCopyDir, err = GetTmpDir(name)
	if err != nil {
		return
	}
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

func GetUnixTimestamp_secondsFromNow(seconds int) int64 {
	return time.Now().Add(time.Duration(seconds) * time.Second).Unix()
}
func GetDuration_fromUnixTimestamp(timestamp int64) (duration time.Duration) {
	duration = 0 * time.Second
	targetTime := time.Unix(timestamp, 0)
	duration = targetTime.Sub(time.Now())
	return
}

func UpackageAppPackage(service *holo.Service, appPackagePath string, toPath string, appName string) (appPackage *holo.AppPackage, err error) {
	sf, err := os.Open(appPackagePath)
	if err != nil {
		return
	}
	defer sf.Close()
	encodingFormat := holo.EncodingFormat(appPackagePath)
	appPackage, err = service.SaveFromAppPackage(sf, toPath, appName, nil, encodingFormat, false)
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
