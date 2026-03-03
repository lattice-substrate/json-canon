package jcstoken_test

import (
	"fmt"
	"log"

	"github.com/lattice-substrate/json-canon/jcstoken"
)

func ExampleParse() {
	v, err := jcstoken.Parse([]byte(`{"name":"Alice","age":30}`))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Kind:", v.Kind)
	fmt.Println("Members:", len(v.Members))
	// Output:
	// Kind: 5
	// Members: 2
}

func ExampleParseWithOptions() {
	opts := &jcstoken.Options{MaxDepth: 2}
	_, err := jcstoken.ParseWithOptions([]byte(`{"a":{"b":{"c":1}}}`), opts)
	fmt.Println("Error:", err != nil)
	// Output: Error: true
}

func ExampleIsNoncharacter() {
	fmt.Println(jcstoken.IsNoncharacter('\uFDD0'))
	fmt.Println(jcstoken.IsNoncharacter('A'))
	// Output:
	// true
	// false
}
