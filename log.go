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
	"time"
)

// Logger holds logger configuration
type Logger struct {
	Enabled bool
	Format  string
	f       string
	tf      string
	color   *color.Color
	w       io.Writer
}

func (l *Logger) setupColor(f string) (colorResult *color.Color, result string) {
	re := regexp.MustCompile(`(.*)\%\{color:([^\}]+)\}(.*)`)
	x := re.FindStringSubmatch(f)
	var txtColor string
	if len(x) > 0 {
		result = x[1] + x[3]
		txtColor = x[2]
	} else {
		result = f
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
		colorResult = color.New(c)
	}
	return
}

func (l *Logger) setupTime(f string) (timeFormat string, result string) {
	re := regexp.MustCompile(`(.*)\%\{time(:[^\}]+)*\}(.*)`)
	x := re.FindStringSubmatch(f)
	if len(x) > 0 {
		result = x[1] + "%{time}" + x[3]
		timeFormat = strings.TrimLeft(x[2], ":")
		if timeFormat == "" {
			timeFormat = time.Stamp
		}
	} else {
		result = f
	}
	return
}

func (l *Logger) New(w io.Writer) (err error) {

	if w == nil {
		l.w = os.Stdout
	} else {
		l.w = w
	}

	if l.Format == "" {
		l.f = `%{message}`
	} else {
		l.color, l.f = l.setupColor(l.Format)
		l.tf, l.f = l.setupTime(l.f)
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
	var t *time.Time
	if l.tf != "" {
		now := time.Now()
		t = &now
	}
	return l._parse(m, t)
}

func (l *Logger) _parse(m string, t *time.Time) (output string) {
	output = strings.Replace(l.f, "%{message}", m, -1)
	if t != nil {
		tTxt := t.Format(l.tf)
		output = strings.Replace(output, "%{time}", tTxt, -1)
	}
	return
}

func (l *Logger) p(m interface{}) {
	l.pf("%v", m)
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
