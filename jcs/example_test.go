package jcs_test

import (
	"fmt"
	"log"

	"github.com/lattice-substrate/json-canon/jcs"
	"github.com/lattice-substrate/json-canon/jcstoken"
)

func ExampleCanonicalize() {
	input := []byte(`{ "b" : 2, "a" : 1 }`)
	canonical, err := jcs.Canonicalize(input)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(canonical))
	// Output: {"a":1,"b":2}
}

func ExampleSerialize() {
	input := []byte(`{ "z" : true, "a" : [3, 1] }`)
	v, err := jcstoken.Parse(input)
	if err != nil {
		log.Fatal(err)
	}
	canonical, err := jcs.Serialize(v)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(canonical))
	// Output: {"a":[3,1],"z":true}
}
