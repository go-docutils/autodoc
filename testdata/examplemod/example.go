// Package example is a fixture for autodoc's own tests, not real code.
package example

// Greet returns a greeting for name.
//
// # Usage
//
// Call it with a plain string:
//
//	Greet("World")
//
// See also [Farewell] and the [Go website] for more.
//
// [Go website]: https://go.dev
func Greet(name string) string {
	return "Hello, " + name
}

// Farewell returns a goodbye for name.
//
// It considers, in order:
//
//  1. the formality level
//  2. the name itself
//
// and beyond that just:
//
//   - says goodbye
//   - moves on
func Farewell(name string) string {
	return "Goodbye, " + name
}

// Greeter says hello and goodbye.
type Greeter struct {
	// Name is who to greet.
	Name string
}

// Hello greets the Greeter's own Name.
func (g *Greeter) Hello() string {
	return Greet(g.Name)
}

// Default levels of greeting formality.
const (
	Casual = "casual"
	Formal = "formal"
)
