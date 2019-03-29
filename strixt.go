// © 2019 Lassi Kortela
// SPDX-License-Identifier: ISC

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	quiet   = 0
	info    = 1
	verbose = 2
)

var verbosity = info
var tabs = false

const MaxTextFileSize = 64 * 1024

// multipleBlankLines
// blankLinesAtStart
// blankLinesAtEnd
// whitespaceAtEol

const asciiControl = "ASCII control character"
const asciiFormFeed = "ASCII page separator ^L"
const carriageReturn = "carriage return"
const nonUtf8Bytes = "invalid UTF-8 sequence"
const tabForAlignment = "tab after non-tab"
const tabsNotAllowed = "tabs not allowed (use -t to allow)"

type Peeve struct {
	lineNo  int
	column  int
	message string
}

func addPeeve(peeves []Peeve, lineNo, column int, message string) []Peeve {
	return append(peeves, Peeve{lineNo, column, message})
}

func readUpToNBytes(filepath string, n int) []byte {
	b := make([]byte, n)
	f, err := os.Open(filepath)
	if err != nil {
		return []byte{}
	}
	defer f.Close()
	nread, err := f.Read(b)
	if err != nil {
		return []byte{}
	}
	return b[:nread]
}

func isBinaryFile(filepath string) bool {
	b := readUpToNBytes(filepath, 100)
	return bytes.Count(b, []byte{0}) > 0
}

func containsNonTabs(s string) bool {
	for _, byte := range s {
		if byte != 0x09 {
			return true
		}
	}
	return false
}

func analyzeTextFile(filepath string) []Peeve {
	ps := []Peeve{}
	b := readUpToNBytes(filepath, MaxTextFileSize)
	ln := 1
	col := 1
	lineText := ""
	for _, byte := range b {
		if byte < 0x09 {
			ps = addPeeve(ps, ln, col, asciiControl)
		} else if byte == 0x09 {
			if !tabs {
				ps = addPeeve(ps, ln, col, tabsNotAllowed)
			}
			if containsNonTabs(lineText) {
				ps = addPeeve(ps, ln, col, tabForAlignment)
			} else {
				lineText += string(byte)
			}
		} else if byte == 0x0a {
			ln++
			col = 1
			lineText = ""
			continue
		} else if byte == 0x0b {
			ps = addPeeve(ps, ln, col, asciiControl)
		} else if byte == 0x0c {
			ps = addPeeve(ps, ln, col, asciiFormFeed)
		} else if byte == 0x0d {
			ps = addPeeve(ps, ln, col, carriageReturn)
		} else if byte < 0x20 {
			ps = addPeeve(ps, ln, col, asciiControl)
		} else if byte == 0x20 {
			// space
		} else if byte < 0x7f {
			// ascii visible
		} else if byte == 0x7f {
			ps = addPeeve(ps, ln, col, asciiControl)
		} else {
		}
		col++
	}
	return ps
}

func walkDir(dirpath string, depth int) {
	ents, err := ioutil.ReadDir(dirpath)
	if err != nil {
		panic(err)
	}
	for _, ent := range ents {
		walkEnt(path.Join(dirpath, ent.Name()), depth+1)
	}
}

func walkEnt(entpath string, depth int) {
	ent, err := os.Lstat(entpath)
	if err != nil {
		panic(err)
	}
	if ent.Mode()&os.ModeSymlink != 0 {
		if depth < 1 || verbosity >= verbose {
			fmt.Printf("%s: skipping %s\n",
				entpath, "symbolic link")
		}
	} else if ent.IsDir() {
		if ent.Name()[0] != '.' || depth < 1 {
			walkDir(entpath, depth)
		} else if depth < 1 || verbosity >= verbose {
			fmt.Printf("%s: skipping %s\n",
				entpath, "hidden directory")
		}
	} else if isBinaryFile(entpath) {
		if depth < 1 || verbosity >= verbose {
			fmt.Printf("%s: skipping %s\n",
				entpath, "binary file")
		}
	} else {
		peeves := analyzeTextFile(entpath)
		if len(peeves) > 0 {
			for _, peeve := range peeves {
				fmt.Printf("%s:%d:%d: error: %s\n",
					entpath, peeve.lineNo, peeve.column,
					peeve.message)
			}
		} else if verbosity >= verbose {
			fmt.Printf("%s: ok\n", entpath)
		}
	}
}

func walk(path string) {
	walkEnt(path, 0)
}

func main() {
	var verboseFlag bool
	flag.BoolVar(&tabs, "t", false, "tabs allowed")
	flag.BoolVar(&verboseFlag, "v", false, "verbose")
	flag.Parse()
	if verboseFlag {
		verbosity = verbose
	}
	entpaths := flag.Args()
	if len(entpaths) == 0 {
		walk(".")
	} else {
		for _, ent := range entpaths {
			walk(ent)
		}
	}
}
