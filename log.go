// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// log encapsulates logging

package holochain

import (
	"fmt"
	"github.com/fatih/color"
	"io"
	"os"
	"regexp"
	"strings"
)

// Logger holds logger configuration
type Logger struct {
	Enabled bool
	Format  string
	f       string
	color   *color.Color
	w       io.Writer
}

func (l *Logger) New(w io.Writer) (err error) {

	if w == nil {
		l.w = os.Stdout
	} else {
		l.w = w
	}
	var f string
	if l.Format == "" {
		f = `%{message}`
	} else {
		f = l.Format
	}

	re := regexp.MustCompile(`^\%\{color:([^\}]+)\}(.*)`)
	x := re.FindStringSubmatch(f)
	var txtColor string
	if len(x) > 0 {
		l.f = x[2]
		txtColor = x[1]
	} else {
		l.f = f
	}

	if txtColor != "" {
		var c color.Attribute
		switch txtColor {
		case "red":
			c = color.FgRed
		case "blue":
			c = color.FgBlue
		case "green":
			c = color.FgGreen
		case "yellow":
			c = color.FgYellow
		case "white":
			c = color.FgWhite
		case "cyan":
			c = color.FgCyan
		case "magenta":
			c = color.FgMagenta
		}
		l.color = color.New(c)
	}

	d := os.Getenv("DEBUG")
	switch d {
	case "1":
		l.Enabled = true
	case "0":
		l.Enabled = false
	}

	return
}

func (l *Logger) parse(m string) (output string) {
	output = strings.Replace(l.f, "%{message}", m, -1)
	return
}

func (l *Logger) p(m interface{}) {
	l.pf("%v\n", m)
}

func (l *Logger) pf(m string, args ...interface{}) {
	if l != nil && l.Enabled {
		f := l.parse(m)
		if l.color != nil {
			l.color.Fprintf(l.w, f+"\n", args...)
		} else {
			fmt.Fprintf(l.w, f+"\n", args...)
		}
	}
}

func (l *Logger) Debug(m interface{}) {
	l.p(m)
}

func (l *Logger) Debugf(m string, args ...interface{}) {
	l.pf(m, args...)
}

func (l *Logger) Info(m interface{}) {
	l.p(m)
}

func (l *Logger) Infof(m string, args ...interface{}) {
	l.pf(m, args...)
}
