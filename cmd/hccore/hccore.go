// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// command line interface to developing and testing holochain applications

// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//---------------------------------------------------------------------------------------
// command line interface to developing and testing holochain applications

package main

import (
	"fmt"
	"github.com/metacurrency/holochain/cmd"
	"github.com/urfave/cli"
	"os"
	"os/exec"

	"bytes"
)

const (
	defaultPort = "4141"
)

func getCurrentDirectoryOrExit() string {
	dir, err := cmd.GetCurrentDirectory()
	if err != nil {
		cmd.Die("HC: hccore.go: getCurrentDirectory: could not find current directory. Weird. Exitting")
	}

	return dir
}

var debug bool
var rootPath, devPath, name string

var fromWhichSourceDirectory, toWhatTargetDirectory string
var noQuestions bool
var compileTargets string

func setupApp() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "hccore"
	app.Usage = "tools for Holochain Core developers"
	// app.Version = fmt.Sprintf("0.0.0 (holochain %s)", holo.VersionStr)
	app.Flags = []cli.Flag{}

	app.Commands = []cli.Command{
		{
			Name: "paradigm",
			// Aliases:    []string{"branch"},
			Usage: "jump into the $GOPATH directory to do stuff like compile and test",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "subDirectory,d",
					Usage:       "navigate to a sub directory to run tests or whatever",
					Value:       getCurrentDirectoryOrExit(),
					Destination: &toWhatTargetDirectory,
				},
			},
			Action: func(c *cli.Context) error {
				action_paradigm()

				return nil
			},
		},
		{
			Name: "fromLocalFilesystem",
			// Aliases:    []string{"branch"},
			Usage: "install a Holochain Core from a local directory (defaults to .)",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "sourceDirectory",
					Usage:       "path to source files containing checked out git branch, defaults to current directory",
					Value:       getCurrentDirectoryOrExit(),
					Destination: &fromWhichSourceDirectory,
				},
			},
			Subcommands: []cli.Command{
				{
					Name: "install",
					// Aliases:    []string{"branch"},
					Usage: "install the version of Holochain Core in '.' onto the host system",
					Flags: []cli.Flag{
						cli.BoolFlag{
							Name:        "noQuestions",
							Usage:       "once the files are made available to Golang, should we compile them?",
							Destination: &noQuestions,
						},
						cli.StringFlag{
							Name:        "compile",
							Usage:       "once the files are made available to Golang, should we compile them?",
							Destination: &compileTargets,
						},
					},
					Action: func(c *cli.Context) error {
						fmt.Printf("HC: core.fromLocalFilesystem.install: installing from        : %v\n", fromWhichSourceDirectory)

						err := os.Chdir(fromWhichSourceDirectory)
						if err != nil {
							fmt.Printf("HC: core.fromLocalFilesystem.install: could not change dir to: %v\n", fromWhichSourceDirectory)
							os.Exit(1)
						}
						fmt.Printf("HC: core.fromLocalFilesystem.install: changed directory to   : %v\n", fromWhichSourceDirectory)

						fmt.Printf("HC: core.fromLocalFilesystem.install: noQuestions, compile   : %v, %v\n", noQuestions, compileTargets)
						// build the script name from the options
						var scriptStringBuffer bytes.Buffer
						scriptStringBuffer.WriteString("holochain.core.fromLocalFilesystem.install")
						if noQuestions {
							scriptStringBuffer.WriteString(".noQuestions")
							if compileTargets != "" {
								scriptStringBuffer.WriteString(".withCompile")
							} else {
								scriptStringBuffer.WriteString(".noCompile")
							}
						}

						// fmt.Printf("HC: core.fromLocalFilesystem.install: running command: %v\n", scriptStringBuffer.String())
						// if silent {
						// maintains the existing go process, and waits for the script to complete
						binpath, err := cmd.GolangHolochainDir("bin", scriptStringBuffer.String())
						if err != nil {
							return err
						}
						fmt.Printf("HC: core.fromLocalFilesystem.install: running command: %v\n", binpath)
						cmd.OsExecPipes(binpath, compileTargets)

						// } else {
						//   // swaps current go process for a(bash)nother process
						//   cmd.ExecBinScript(scriptStringBuffer.String())
						// }

						return nil
					},
				},
			},
		},
	}

	app.Action = func(c *cli.Context) error {
		cli.ShowAppHelp(c)

		return nil
	}

	return app
}

func action_paradigm() *exec.Cmd {
	targetDirecgtory, err := cmd.GolangHolochainDir(os.Args[2])
	if err != nil {
		panic("could not find target directory")
	}
	os.Chdir(targetDirecgtory)
	return cmd.OsExecPipes("bash")
}

func main() {
	app := setupApp()

	err := app.Run(os.Args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
