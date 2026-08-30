// Copyright (c) the go-docutils/autodoc authors.
// SPDX-License-Identifier: BSD-3-Clause

package autodoc_test

import (
	"strings"
	"testing"

	"github.com/go-docutils/autodoc"
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
		"Hello\n~~~~~", // a method nests one level deeper than its type
		"func (g *Greeter) Hello() string",
		"Casual, Formal\n--------------",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Generate output missing %q:\n%s", want, src)
		}
	}
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
