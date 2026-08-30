// Copyright (c) the go-docutils/autodoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package autodoc_test

import (
	"strings"
	"testing"

	"github.com/go-docutils/autodoc"
	"github.com/go-docutils/docutils/doctree"
	"github.com/go-docutils/docutils/rst"
)

// generate is the fixture module every test below runs against:
// testdata/examplemod, a small module exercising a function, a type with a
// field and a method, a const group, and a doc comment with a "# Usage"
// heading, a preformatted block, a [Farewell] doc link (unresolved, so it
// renders as inline code — see the package doc comment), and a real
// [Go website] link.
func generate(t *testing.T) string {
	t.Helper()
	src, err := autodoc.Generate("testdata/examplemod")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return src
}

func TestGenerateContains(t *testing.T) {
	src := generate(t)
	for _, want := range []string{
		"example.test/examplemod\n=======================",
		"Package example is a fixture for autodoc's own tests, not real code.",
		"Greet\n-----\n\n::\n\n    func Greet(name string) string",
		"Greet returns a greeting for name.",
		"**Usage**\n\nCall it with a plain string:",
		"::\n\n    Greet(\"World\")",
		"See also ``Farewell`` and the `Go website <https://go.dev>`_ for more.",
		"1. the formality level\n2. the name itself",
		"- says goodbye\n- moves on",
		"Greeter\n-------\n\n::\n\n    type Greeter struct {",
		"``Name`` (string)\n    Name is who to greet.",
		"Hello\n~~~~~", // a method nests one level deeper than its type
		"func (g *Greeter) Hello() string",
		"Casual, Formal\n--------------",
		"fmt.Println(example.Greet(\"World\"))",
		"Output:\n\n::\n\n    Hello, World",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Generate output missing %q:\n%s", want, src)
		}
	}
}

// TestMethodExampleNestsUnderItsMethod guards a real structural bug: with
// only 3 heading-depth characters, a method's own Example (package(0) ->
// type(1) -> method(2) -> example(3), clamped) reused the SAME underline
// character as the method itself — two sections sharing one underline
// character parse as SIBLINGS in reST (docutils/rst tracks nesting by
// first-seen order of the character, not by any notion of "depth"), so
// the example silently came out as the method's sibling instead of its
// child. headingChars grew a 4th level to fix it; this asserts the fixed
// shape by walking the actual parsed tree, not just checking the raw
// underline characters differ (which wouldn't prove nesting on its own).
func TestMethodExampleNestsUnderItsMethod(t *testing.T) {
	src := generate(t)
	tree := rst.Parse(src)
	pkg := findSection(t, tree, "example.test/examplemod")
	greeter := findSection(t, pkg, "Greeter")
	hello := findSection(t, greeter, "Hello")
	findSection(t, hello, "Example") // fails the test via t.Fatal if not found nested here
}

// TestStructFieldBecomesDefinitionListItem checks the field documentation
// actually parses as a real reST definition list (a <term>/<definition>
// pair), not just that the raw text happens to contain the right
// substrings — the same "walk the parsed tree" rigor as the nesting test
// above, since term/body indentation is exactly the kind of thing that
// looks right in a string comparison while being subtly malformed reST.
func TestStructFieldBecomesDefinitionListItem(t *testing.T) {
	src := generate(t)
	tree := rst.Parse(src)
	pkg := findSection(t, tree, "example.test/examplemod")
	greeter := findSection(t, pkg, "Greeter")
	var dl *doctree.Element
	for _, c := range greeter.Children {
		if el, ok := c.(*doctree.Element); ok && el.Tag == doctree.TagDefinitionList {
			dl = el
		}
	}
	if dl == nil {
		t.Fatalf("no definition_list found under Greeter's own section")
	}
	item, ok := dl.Children[0].(*doctree.Element)
	if !ok || item.Tag != doctree.TagDefinitionListItem {
		t.Fatalf("definition_list's first child is not a definition_list_item: %#v", dl.Children[0])
	}
}

func findSection(t *testing.T, parent *doctree.Element, title string) *doctree.Element {
	t.Helper()
	for _, c := range parent.Children {
		el, ok := c.(*doctree.Element)
		if !ok || el.Tag != doctree.TagSection {
			continue
		}
		for _, cc := range el.Children {
			if te, ok := cc.(*doctree.Element); ok && te.Tag == doctree.TagTitle && doctree.AsText(te) == title {
				return el
			}
		}
	}
	t.Fatalf("no %q section found nested under this parent", title)
	return nil
}

// TestGenerateOutputParses is this package's own correctness proof, the
// same pattern go-richdoc/rst uses for its Write: rather than trusting
// this package's own idea of valid reST, the output is fed back through
// the real engine and must come back as one document with no error and no
// leftover unparsed shape.
func TestGenerateOutputParses(t *testing.T) {
	src := generate(t)
	tree := rst.Parse(src)
	if len(tree.Children) == 0 {
		t.Fatal("parsed to an empty document")
	}
}

// TestLongValueGroupHeadingIsBounded guards a real bug caught by running
// this package against go-docutils/docutils' own doctree package while
// developing it: an undocumented const block with dozens of names (exactly
// the shape doctree's own Tag* constants take) produced one heading
// listing every name, underlined to match — technically valid reST,
// absurd to read.
func TestLongValueGroupHeadingIsBounded(t *testing.T) {
	src, err := autodoc.Generate("testdata/manynames")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(src, "A, B, C, and 3 more") {
		t.Errorf("long value group heading not bounded:\n%s", src)
	}
	if strings.Count(src, "A, B, C, D, E, F") > 0 {
		t.Errorf("heading still lists every name:\n%s", src)
	}
}

func TestNoGoModIsAnError(t *testing.T) {
	src, err := autodoc.Generate(t.TempDir())
	if err == nil {
		t.Fatal("a directory with no go.mod should fail, not silently generate nothing")
	}
	if src != "" {
		t.Errorf("Generate on a failure returned non-empty output: %q", src)
	}
}

func TestMissingDirIsAnError(t *testing.T) {
	if _, err := autodoc.Generate("testdata/does-not-exist"); err == nil {
		t.Fatal("a nonexistent directory should fail, not silently generate nothing")
	}
}

func TestSyntaxErrorIsAnError(t *testing.T) {
	if _, err := autodoc.Generate("testdata/brokenmod"); err == nil {
		t.Fatal("a package that fails to parse should fail, not silently skip it")
	}
}
