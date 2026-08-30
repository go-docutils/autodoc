// Copyright (c) the go-docutils/autodoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package autodoc

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/printer"
	"go/token"
	"strings"
	"unicode/utf8"
)

// Section-underline characters, one per nesting depth: package, then
// func/type/const-or-var group, then a type's own methods — three levels
// deep is as far as this package's own output ever nests, so unlike
// docutils/rst's own first-seen-order tracking (arbitrary depth), a fixed
// table is enough here.
var headingChars = []byte{'=', '-', '~'}

func renderPackage(fset *token.FileSet, pkg *doc.Package) string {
	var b strings.Builder
	writeHeading(&b, pkg.ImportPath, 0)
	if pkg.Doc != "" {
		b.WriteString(renderCommentDoc(pkg.Doc))
	}
	for _, fn := range pkg.Funcs {
		renderFunc(&b, fset, fn, 1)
	}
	for _, t := range pkg.Types {
		renderType(&b, fset, t)
	}
	for _, v := range pkg.Consts {
		renderValue(&b, fset, v)
	}
	for _, v := range pkg.Vars {
		renderValue(&b, fset, v)
	}
	return b.String()
}

func renderFunc(b *strings.Builder, fset *token.FileSet, fn *doc.Func, depth int) {
	writeHeading(b, fn.Name, depth)
	writeCodeBlock(b, declText(fset, fn.Decl))
	if fn.Doc != "" {
		b.WriteString(renderCommentDoc(fn.Doc))
	}
}

func renderType(b *strings.Builder, fset *token.FileSet, t *doc.Type) {
	writeHeading(b, t.Name, 1)
	writeCodeBlock(b, declText(fset, t.Decl))
	if t.Doc != "" {
		b.WriteString(renderCommentDoc(t.Doc))
	}
	for _, fn := range t.Funcs {
		renderFunc(b, fset, fn, 2)
	}
	for _, m := range t.Methods {
		renderFunc(b, fset, m, 2)
	}
}

func renderValue(b *strings.Builder, fset *token.FileSet, v *doc.Value) {
	writeHeading(b, valueHeading(v), 1)
	writeCodeBlock(b, declText(fset, v.Decl))
	if v.Doc != "" {
		b.WriteString(renderCommentDoc(v.Doc))
	}
}

// valueHeading names a const/var group. Joining every name works for the
// common case (one or a handful sharing a single doc comment), but a large
// undocumented block — TagXxx-style constants are exactly this shape —
// would otherwise produce a heading dozens of names long, underlined to
// match: technically valid reST, absurd to read. Past a small cap it
// names just the first couple and says how many more there are instead.
func valueHeading(v *doc.Value) string {
	const shown = 3
	if len(v.Names) <= shown+1 {
		return strings.Join(v.Names, ", ")
	}
	return strings.Join(v.Names[:shown], ", ") + fmt.Sprintf(", and %d more", len(v.Names)-shown)
}

// declText renders a declaration node back to Go source with go/printer —
// the same formatter `gofmt` itself uses, so this always comes out
// gofmt-clean regardless of how the original source was laid out.
func declText(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return fmt.Sprintf("%v", node) // unreachable for a well-formed *ast.FuncDecl/*ast.GenDecl; defensive only
	}
	return buf.String()
}

func writeHeading(b *strings.Builder, title string, depth int) {
	ch := headingChars[depth]
	if depth >= len(headingChars) {
		ch = headingChars[len(headingChars)-1]
	}
	b.WriteString(title + "\n")
	b.WriteString(strings.Repeat(string(ch), utf8.RuneCountInString(title)) + "\n\n")
}

func writeCodeBlock(b *strings.Builder, code string) {
	b.WriteString("::\n\n")
	for _, line := range strings.Split(strings.TrimRight(code, "\n"), "\n") {
		if line == "" {
			b.WriteString("\n")
		} else {
			b.WriteString("    " + line + "\n")
		}
	}
	b.WriteString("\n")
}

// renderCommentDoc renders a Go doc comment's parsed structure (go/doc/
// comment: headings, paragraphs, lists, preformatted blocks, links) as
// reST — the structural analogue of what Sphinx's napoleon extension does
// for a free-form Python docstring, except Go's own doc-comment syntax is
// already structured, so there is only a rendering step, no ad hoc
// convention-guessing.
func renderCommentDoc(text string) string {
	// LookupSym unconditionally true: this package doesn't resolve doc
	// links to real symbols (see the package doc comment, "NOT
	// implemented: cross-symbol navigation"), but still wants [Foo]
	// bracket syntax recognized as a link — the alternative, LookupSym
	// nil, would leave every such reference as literal, unrendered
	// brackets instead of the visually-distinct inline code the
	// *comment.DocLink case below produces.
	p := &comment.Parser{LookupSym: func(recv, name string) bool { return true }}
	d := p.Parse(text)
	var b strings.Builder
	for _, block := range d.Content {
		renderCommentBlock(&b, block)
	}
	return b.String()
}

func renderCommentBlock(b *strings.Builder, block comment.Block) {
	switch v := block.(type) {
	case *comment.Paragraph:
		b.WriteString(renderCommentText(v.Text))
		b.WriteString("\n\n")
	case *comment.Heading:
		// A doc comment heading nested inside a func/type's own reST
		// section: rendered as emphasized text rather than a further
		// section level, since a comment can put a heading anywhere and
		// reST section nesting must stay strictly ordered — safer than
		// guessing a consistent depth for content this package doesn't
		// otherwise track levels for.
		b.WriteString("**" + renderCommentText(v.Text) + "**\n\n")
	case *comment.Code:
		writeCodeBlock(b, v.Text)
	case *comment.List:
		for _, item := range v.Items {
			marker := "-"
			if item.Number != "" {
				marker = item.Number + "."
			}
			var lines []string
			for _, p := range item.Content {
				if para, ok := p.(*comment.Paragraph); ok {
					lines = append(lines, renderCommentText(para.Text))
				}
			}
			b.WriteString(marker + " " + strings.Join(lines, " ") + "\n")
		}
		b.WriteString("\n")
	}
}

func renderCommentText(texts []comment.Text) string {
	var b strings.Builder
	for _, t := range texts {
		switch v := t.(type) {
		case comment.Plain:
			b.WriteString(escapeRST(string(v)))
		case comment.Italic:
			b.WriteString("*" + escapeRST(string(v)) + "*")
		case *comment.Link:
			b.WriteString("`" + renderCommentText(v.Text) + " <" + v.URL + ">`_")
		case *comment.DocLink:
			// No cross-symbol navigation in v1 (see the package doc
			// comment) — rendered as inline code rather than a link to
			// nowhere, still visually distinct from ordinary prose.
			b.WriteString("``" + renderCommentText(v.Text) + "``")
		}
	}
	return b.String()
}

// escapeRST backslash-escapes the characters reST's inline-markup grammar
// treats specially at a word boundary, so plain doc-comment prose (which
// knows nothing about reST) can't accidentally trigger emphasis, a
// substitution reference, or an inline target by coincidence.
func escapeRST(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '*', '`', '|', '_':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
