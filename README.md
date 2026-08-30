# go-docutils/autodoc

Turns a Go module's exported API into reStructuredText, feeding
[`go-docutils/docutils`](https://github.com/go-docutils/docutils)'s own
parser/writers the way Sphinx's `autodoc` extension feeds docutils in the
Python world — but by walking real Go source with the standard library's
own `go/doc` and `go/doc/comment`, not by importing and introspecting live
code the way `autodoc` does.

That difference is the whole reason this exists as a separate package
rather than inside `docutils` itself: `docutils`' own README explicitly
keeps Python-introspection-coupled tooling out of scope, but a Go-native
equivalent has no such coupling to avoid — Go's own doc-comment syntax
(headings, lists, links, preformatted blocks; see `go/doc/comment`) is
already the structured format Sphinx's `napoleon` extension exists to
retrofit onto free-form Python docstrings, so there is no `napoleon`-shaped
gap here to fill separately.

```go
import "github.com/go-docutils/autodoc"

src, err := autodoc.Generate("/path/to/a/go/module")
// src is reST source; feed it to docutils/rst.Parse, or further to
// docutils/html or docutils/latex, or to go-richdoc/rst.
```

## Why this repo, not a new org

`go-docutils`'s own founding rationale reserved exactly this: `docutils`
names the parser+doctree+writer *layer*, leaving room for a future
Sphinx-specific consumer layer on top of it — the same pattern
[`go-tex-typeset-libs`](https://github.com/go-tex) lives under `go-tex`
rather than its own org. `autodoc` (and a future `napoleon`, if Go ever
grows a structured-docstring convention worth retrofitting the way
Python's Google/NumPy styles do) is that reserved layer.

## Scope (v1)

`Generate` walks every package under a module root and documents its
exported functions, types (with their exported methods), and constant/
variable groups — each package as its own top-level section, in directory
order, one flat reST document. `docutils/rst` has no toctree/multi-file
project concept to build a real Sphinx-style multi-page site on top of, so
this produces the single-document equivalent: everything one real
LaTeX/HTML compile can consume, matching how
[`go-richdoc/rst/pdf`](https://github.com/go-richdoc/rst) already proves a
document all the way to a real PDF (this package's own tests prove the
same thing, against `docutils`' own source as the corpus).

A doc comment's structure — headings, lists, preformatted code, links — is
rendered properly via `go/doc/comment`, not flattened to plain prose. A
`[Symbol]` doc link renders as inline code rather than a cross-reference:
there is nowhere for it to point without a real multi-file site (v1 has
none), so it stays visually distinct from ordinary prose instead of either
resolving to nothing or vanishing into it.

**Not implemented**: cross-symbol navigation (see above), Examples
(`go/doc`'s own field, extracted from `_test.go` files), and per-field
struct documentation (a type's declaration is shown verbatim instead,
which already carries field doc comments as ordinary Go comments —
readable, just not individually reST-structured). A large undocumented
const/var group (more than four names sharing one block, `doctree`'s own
`Tag*` constants being exactly this shape) gets a bounded heading — the
first three names and a count — rather than every name joined into one
absurdly long, dash-underlined title.

## Testing

`go test ./...` runs against `testdata/examplemod`, a small fixture module
exercising every construct above, and asserts on `Generate`'s exact output
for each. The real correctness proof is `TestGenerateOutputParses`: rather
than trusting this package's own idea of valid reST, the output is fed
back through `docutils/rst.Parse` (this package's only non-stdlib
dependency, test-only) and must come back as a real document — the same
pattern `go-richdoc/rst`'s `Write` uses for itself. `go vet ./...` and
`gofmt -l .` (excluding `testdata`, which intentionally holds one
syntactically invalid fixture) are clean.

## License

BSD-3-Clause. See [LICENSE](LICENSE).
