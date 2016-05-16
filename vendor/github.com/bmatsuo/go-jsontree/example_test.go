package jsontree

import (
	"fmt"
)

func Example() {
	raw := `{
		"null":   null,
		"object": {
			"int":   123,
			"bool":  false,
			"array": [
				"string",
				null
			]
		}
	}`
	tree := New()
	err := tree.UnmarshalJSON([]byte(raw))
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(tree.Get("object").Get("array").GetIndex(0).String())
	fmt.Println(tree.Get("object").Get("int").Number())
	fmt.Println(tree.Get("doesntexist").Err())
	fmt.Println(tree.Get("null").Err())
	fmt.Println(tree.Get("null").IsNull())
	// Output: string <nil>
	// 123 <nil>
	// key does not exist; $.doesntexist
	// <nil>
	// true
}
