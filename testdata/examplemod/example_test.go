package example_test

import (
	"fmt"

	"example.test/examplemod"
)

// ExampleGreet shows Greet in action.
func ExampleGreet() {
	fmt.Println(example.Greet("World"))
	// Output: Hello, World
}

func ExampleGreeter_Hello() {
	g := &example.Greeter{Name: "Ann"}
	fmt.Println(g.Hello())
	// Output: Hello, Ann
}
