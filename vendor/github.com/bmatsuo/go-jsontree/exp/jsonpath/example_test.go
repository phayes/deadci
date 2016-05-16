// Copyright 2013, Bryan Matsuo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// jsonpath_test.go [created: Mon, 10 Jun 2013]

package jsonpath

import (
	"github.com/bmatsuo/go-jsontree"

	"fmt"
)

func Example() {
	raw := `[
		{
			"name":{
				"first":"alice"
			},
			"phone":"555-1234"
		},
		{
			"name":{
				"first":"bob"
			},
			"phone":"555-4321"
		},
		{
			"name":{
				"first":"bob"
			},
			"phone":"555-3513"
		},
		{
			"name":{
				"first":"carol"
			},
			"phone":"555-9352"
		}
	]`
	js := jsontree.New()
	err := js.UnmarshalJSON([]byte(raw))
	if err != nil {
		panic(err)
	}
	// $.*[name.first = 'bob'].phone
	bobsNumbers := Lookup(js,
		All,
		EqualString("bob",
			Key("name"),
			Key("first")),
		Key("phone"))
	for _, number := range bobsNumbers {
		str, _ := number.String()
		fmt.Println(str)
	}
	// Output:
	// 555-4321
	// 555-3513
}
