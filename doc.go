// Copyright (c) the go-docutils/autodoc authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package autodoc turns a Go module's exported API into reStructuredText,
// feeding [github.com/go-docutils/docutils]'s own parser/writers the same
// way Sphinx's autodoc extension feeds docutils in the Python world — but
// by walking real Go source with the standard library's own go/doc and
// go/doc/comment, not by importing and introspecting live code the way
// autodoc does. That difference is the whole reason this exists as a
// separate package rather than inside docutils itself: docutils' own
// README explicitly keeps Python-introspection-coupled tooling out of
// scope, but a Go-native equivalent has no such coupling to avoid — Go's
// own doc-comment syntax (headings, lists, links, preformatted blocks; see
// go/doc/comment) is ALREADY the structured format Sphinx's napoleon
// extension exists to retrofit onto free-form Python docstrings, so there
// is no napoleon-shaped gap here to fill separately.
//
// SCOPE (v1): [Generate] walks every package under a module root and
// documents its exported functions, types (with their exported methods),
// and constant/variable groups — each package as its own top-level
// section, in directory order, one flat reST document (docutils/rst has
// no toctree/multi-file project concept to build a real Sphinx-style
// multi-page site on top of, so this produces the single-document
// equivalent: everything one real LaTeX/HTML compile can consume, matching
// how [github.com/go-richdoc/rst/pdf] already proves a document all the
// way to a real PDF). A doc comment's structure (headings, lists,
// preformatted code, links) is rendered properly via go/doc/comment, not
// flattened to plain prose. NOT implemented: cross-symbol navigation (a
// [T] doc link renders as inline code, not a cross-reference — there is
// nowhere for it to point without a real multi-file site), examples
// (go/doc's own Examples field, extracted from _test.go files), and
// interface/struct field-level documentation (a type's declaration is
// shown verbatim instead, which already carries field doc comments as
// ordinary Go comments — readable, just not individually reST-structured).
package autodoc

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Generate walks the Go module rooted at dir and returns reST source
// documenting every package's exported API.
func Generate(dir string) (string, error) {
	modulePath, err := readModulePath(dir)
	if err != nil {
		return "", err
	}
	pkgDirs, err := discoverPackageDirs(dir)
	if err != nil {
		return "", err
	}
	var sections []string
	for _, pd := range pkgDirs {
		importPath := modulePath
		if rel, err := filepath.Rel(dir, pd); err == nil && rel != "." {
			importPath = modulePath + "/" + filepath.ToSlash(rel)
		}
		src, err := renderPackageDir(pd, importPath)
		if err != nil {
			return "", fmt.Errorf("autodoc: %s: %w", importPath, err)
		}
		if src != "" {
			sections = append(sections, src)
		}
	}
	return strings.Join(sections, "\n"), nil
}

// readModulePath reads the module path out of dir/go.mod's own "module "
// line — a full go.mod parse (golang.org/x/mod/modfile) would be more
// robust, but pulls in a dependency for one line this project's own
// zero-dependency convention (see docutils itself) doesn't otherwise need.
func readModulePath(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("autodoc: reading go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("autodoc: %s has no module line", filepath.Join(dir, "go.mod"))
}

// discoverPackageDirs finds every directory under root holding at least one
// non-test .go file, skipping the conventional "not really package code"
// directories: dot-directories, "_"-prefixed directories (Go's own
// build-exclusion convention), "testdata", and "vendor" (a dependency's own
// docs aren't this module's API). Sorted so output order is stable.
func discoverPackageDirs(root string) ([]string, error) {
	var dirs []string
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
				name == "testdata" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go") {
			dir := filepath.Dir(path)
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs, err
}

// renderPackageDir parses one directory's non-test .go files and renders
// its exported API. Returns "" (no error) for a directory holding only
// files that failed the package name filter (mixed build-tag variants
// this simple a walk doesn't try to resolve — a real go/packages-based
// tool would, at the cost of the dependency this one avoids).
func renderPackageDir(dir, importPath string) (string, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var files []*ast.File
	var pkgName string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return "", fmt.Errorf("parsing %s: %w", name, err)
		}
		if pkgName == "" {
			pkgName = f.Name.Name
		} else if f.Name.Name != pkgName {
			continue // a second package in the same dir (e.g. a `_test` package here by mistake) — skip it
		}
		files = append(files, f)
	}
	if len(files) == 0 || pkgName == "main" {
		return "", nil
	}
	pkg, err := doc.NewFromFiles(fset, files, importPath)
	if err != nil {
		return "", fmt.Errorf("building doc: %w", err)
	}
	return renderPackage(fset, pkg), nil
}
