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
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Logger holds logger configuration
type Logger struct {
	Name    string
	Enabled bool
	Format  string
	f       string
	tf      string
	color   *color.Color
	w       io.Writer

	Prefix      string
	PrefixColor *color.Color
}

var colorMap map[string]*color.Color
var EnableAllLoggersEnv string = "HC_ENABLE_ALL_LOGS"

func (h *Logger) GetColor(colorName string) *color.Color {
	if _, ok := colorMap["red"]; !ok {
		colorMap = make(map[string]*color.Color)
		colorMap["red"] = color.New(color.FgRed)
		colorMap["blue"] = color.New(color.FgBlue)
		colorMap["green"] = color.New(color.FgGreen)
		colorMap["yellow"] = color.New(color.FgYellow)
		colorMap["white"] = color.New(color.FgWhite)
		colorMap["cyan"] = color.New(color.FgCyan)
		colorMap["magenta"] = color.New(color.FgMagenta)
	}
	if val, ok := colorMap[colorName]; ok {
		return val
	} else {
		return colorMap["white"]
	}
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
		colorResult = l.GetColor(txtColor)
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
		l.color = nil
	} else {
		l.color, l.f = l.setupColor(l.Format)
		l.tf, l.f = l.setupTime(l.f)
	}

	d := os.Getenv(EnableAllLoggersEnv)
	switch d {
	case "1":
		l.Enabled = true
	case "0":
		l.Enabled = false
	}

	return
}

func (l *Logger) SetPrefix(prefixFormat string) {
	l.PrefixColor, l.Prefix = l.setupColor(prefixFormat)
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

	// TODO add the calling depth to the line format string.
	re := regexp.MustCompile(`(%{line})|(%{file})`)
	matches := re.FindStringSubmatch(l.f)
	if len(matches) > 0 {
		_, file, line, ok := runtime.Caller(6)
		if ok {
			// sometimes the stack is one less deep than we expect in which case
			// the file shows "asm_" so check for this case and redo!
			if strings.Index(file, "asm_") > 0 {
				_, file, line, ok = runtime.Caller(5)
			}
			output = strings.Replace(output, "%{file}", filepath.Base(file), -1)
			output = strings.Replace(output, "%{line}", fmt.Sprintf("%d", line), -1)
		}
	}
	return
}

func (l *Logger) p(m interface{}) {
	l.pf("%v", m)
}

func (l *Logger) pf(m string, args ...interface{}) {
	if l != nil && l.Enabled {
		l.prefixPrint()
		f := l.parse(m)
		if l.color != nil {
			l.color.Fprintf(l.w, f+"\n", args...)
		} else {
			fmt.Fprintf(l.w, f+"\n", args...)
		}
	}
}

func (l *Logger) prefixPrint(args ...interface{}) {
	if l.PrefixColor != nil {
		l.PrefixColor.Fprintf(l.w, l.Prefix, args...)
	} else {
		fmt.Fprintf(l.w, l.Prefix, args...)
	}
}

func (l *Logger) Log(m interface{}) {
	l.p(m)
}

func (l *Logger) Logf(m string, args ...interface{}) {
	l.pf(m, args...)
}
