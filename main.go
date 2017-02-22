package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

var rewrite bool
var infile string

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [flags] PATH\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.BoolVar(&rewrite, "w", false, "write result to (source) file instead of stdout")
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		return
	}

	fileName := flag.Arg(0)

	fset := token.NewFileSet()

	// Load this file, including comments
	f, err := parser.ParseFile(fset, fileName, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	ast.Walk(&walker{fset: fset}, f)

	var buf bytes.Buffer
	err = format.Node(&buf, fset, f)
	if err != nil {
		log.Fatal(err)
	}
	if rewrite {
		tmpfile, err := ioutil.TempFile("", fileName)
		if err != nil {
			log.Fatal(err)
		}
		defer os.Remove(tmpfile.Name()) // Clean up in case of error

		// Write to tempfile
		if _, err = buf.WriteTo(tmpfile); err != nil {
			log.Fatal(err)
		}
		if err = tmpfile.Close(); err != nil {
			log.Fatal(err)
		}

		// Move tempfile over top of previous file
		if err = os.Rename(tmpfile.Name(), fileName); err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Printf("%s", buf.String())
	}
}

type walker struct {
	fset     *token.FileSet
	lastLine int
	indent   string
}

// Visit tracks current indentation and pretty-prints json multi-line strings
func (w *walker) Visit(node ast.Node) ast.Visitor {
	nextw := w
	if node != nil {
		pos := w.fset.Position(node.Pos())
		if pos.Line != w.lastLine {
			// The returned Visitor is used to recurse into child nodes, so a new copy is created when
			// changing the indent so it doesn't have to be "un-changed" as the recursion unwinds.
			nextw = &walker{
				fset:     w.fset,
				lastLine: pos.Line,
				indent:   strings.Repeat("\t", pos.Column-1), // Assuming the file is all indented with tabs
			}
		}
	}
	switch t := node.(type) {
	case *ast.BasicLit:
		if t.Kind == token.STRING && len(t.Value) >= 4 && t.Value[0] == '`' {
			trimmed := t.Value[1 : len(t.Value)-1]
			trimmed = strings.TrimSpace(trimmed)
			if trimmed[0] == '{' {
				t.Value = fmt.Sprintf("`%s`", prettyJSON(trimmed, nextw.indent))
			}
		}
	case nil:
	default:

	}
	return nextw
}

func prettyJSON(j, indent string) string {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(j), indent, "\t"); err != nil {
		fmt.Println(err)
		return j
	}
	return pretty.String()
}
