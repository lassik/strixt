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
const MaxShownPeevesPerFile = 10

const asciiControl = "ASCII control character"
const asciiFormFeed = "ASCII page separator ^L"
const carriageReturn = "file contains carriage return"
const nonUtf8Bytes = "invalid UTF-8 sequence"
const tabForAlignment = "tabs not allowed after non-tab characters on a line"
const tabsNotAllowed = "tabs not allowed (use -t to allow)"

const whitespaceOnBlankLine = "invisible whitespace on blank line"
const whitespaceAtEndOfLine = "invisible whitespace at end of line"
const blankLineAtStartOfFile = "blank line(s) at beginning of file"
const blankLineAtEndOfFile = "blank line(s) at end of file"
const noNewlineAtEndOfFile = "no newline at end of file"

const maxLineLength = 79
const lineTooLong = "line longer than 79 characters"

const maxConsecutiveBlankLines = 2
const tooManyBlankLines = "more than 2 consecutive blank lines"

type Peeve struct {
	humanLn  int
	humanCol int
	message  string
}

func addPeeve(peeves []Peeve, humanLn, humanCol int, message string) []Peeve {
	return append(peeves, Peeve{humanLn, humanCol, message})
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

func isSpaceByte(c byte) bool {
	return c == 0x09 || c == 0x20
}

func analyzeTextFile(filepath string) []Peeve {
	ps := []Peeve{}
	b := readUpToNBytes(filepath, MaxTextFileSize)
	hadAnyNonblankLines := false
	consecutiveBlankLines := 0
	humanLn := 0
	humanCol := 0
	lineLeadingTabs := 0
	lineLeadingSpaces := 0
	lineMiscLeadingWhite := 0
	lineStartByteOffset := 0
	isNewline := true
	for byteOffset, byte := range b {
		if isNewline {
			// beginning of line
			humanLn++
			humanCol = 1
			lineLeadingTabs = 0
			lineLeadingSpaces = 0
			lineMiscLeadingWhite = 0
			lineStartByteOffset = byteOffset
		}
		// beginning of new byte on line
		byteHumanColWidth := 1
		byteOffsetOnLine := byteOffset - lineStartByteOffset
		isNewline = false
		if byte < 0x09 {
			ps = addPeeve(ps, humanLn, humanCol, asciiControl)
		} else if byte == 0x09 {
			if !tabs {
				ps = addPeeve(ps, humanLn, humanCol,
					tabsNotAllowed)
			}
			const maxTabWidth = 8
			byteHumanColWidth = maxTabWidth -
				((humanCol - 1) % maxTabWidth)
			if byteOffsetOnLine == lineLeadingTabs {
				lineLeadingTabs++
			} else {
				if tabs {
					ps = addPeeve(ps, humanLn, humanCol,
						tabForAlignment)
				}
				if byteOffsetOnLine ==
					lineLeadingTabs+lineLeadingSpaces {
					lineMiscLeadingWhite++
				}
			}
		} else if byte == 0x0a {
			isNewline = true
		} else if byte == 0x0b {
			ps = addPeeve(ps, humanLn, humanCol, asciiControl)
		} else if byte == 0x0c {
			ps = addPeeve(ps, humanLn, humanCol, asciiFormFeed)
		} else if byte == 0x0d {
			ps = addPeeve(ps, humanLn, humanCol, carriageReturn)
		} else if byte < 0x20 {
			ps = addPeeve(ps, humanLn, humanCol, asciiControl)
		} else if byte == 0x20 {
			if byteOffsetOnLine ==
				lineLeadingTabs+lineLeadingSpaces {
				lineLeadingSpaces++
			} else if byteOffsetOnLine ==
				lineLeadingTabs+
					lineLeadingSpaces+
					lineMiscLeadingWhite {
				lineMiscLeadingWhite++
			}
		} else if byte < 0x7f {
			// ascii visible
		} else if byte == 0x7f {
			ps = addPeeve(ps, humanLn, humanCol, asciiControl)
		}
		if !isNewline {
			humanCol += byteHumanColWidth
			continue
		}
		// end of line
		lineLengthBytes := byteOffsetOnLine
		isBlankLine := lineLengthBytes ==
			lineLeadingTabs+lineLeadingSpaces+lineMiscLeadingWhite
		if isBlankLine {
			consecutiveBlankLines++
			if !hadAnyNonblankLines {
				if consecutiveBlankLines == 1 {
					ps = addPeeve(ps, humanLn, humanCol,
						blankLineAtStartOfFile)
				}
			} else if consecutiveBlankLines >
				maxConsecutiveBlankLines {
				ps = addPeeve(ps, humanLn, humanCol,
					tooManyBlankLines)
			}
		} else {
			consecutiveBlankLines = 0
			hadAnyNonblankLines = true
		}
		if humanCol > maxLineLength {
			ps = addPeeve(ps, humanLn, humanCol, lineTooLong)
		}
		if isBlankLine && lineLengthBytes > 0 {
			ps = addPeeve(ps, humanLn, humanCol,
				whitespaceOnBlankLine)
		}
		if !isBlankLine && isSpaceByte(b[byteOffset-1]) {
			ps = addPeeve(ps, humanLn, humanCol,
				whitespaceAtEndOfLine)
		}
	}
	// end of file
	if consecutiveBlankLines > 0 {
		ps = addPeeve(ps, humanLn, humanCol, blankLineAtEndOfFile)
	}
	if !isNewline {
		ps = addPeeve(ps, humanLn, humanCol, noNewlineAtEndOfFile)
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
			for i, peeve := range peeves {
				if i >= MaxShownPeevesPerFile {
					fmt.Printf("%s: too many errors\n",
						entpath)
					break
				}
				fmt.Printf("%s:%d:%d: error: %s\n",
					entpath,
					peeve.humanLn,
					peeve.humanCol,
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
