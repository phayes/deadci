// Copyright 2013, Bryan Matsuo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// jsonpath.go [created: Mon, 10 Jun 2013]

/*
jsonpath is an experimental package for querying jsontree structures. it
generally conforms with the dot-syntax of JSONPath

	http://goessner.net/articles/JsonPath/

Warning

jsonpath is an experimental package and it's API is subject to change without
notice.
*/
package jsonpath

import (
	"github.com/bmatsuo/go-jsontree"
)

func Lookup(js *jsontree.JsonTree, path ...Selector) []*jsontree.JsonTree {
	var selected []*jsontree.JsonTree
	jschan := make(chan *jsontree.JsonTree, 2)
	switch len(path) {
	case 0:
		return nil
	case 1:
		go path[0](jschan, js)
	default:
		go Chain(path...)(jschan, js)
	}
	for js := range jschan {
		if js == nil {
			break
		}
		selected = append(selected, js)
	}
	return selected
}

// Selectors MUST send nil on the channel when there are no more elements.
type Selector func(chan<- *jsontree.JsonTree, *jsontree.JsonTree)

func Chain(path ...Selector) Selector {
	return func(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
		cin := make(chan *jsontree.JsonTree, 2)
		cin <- js
		cin <- nil
		cout := make(chan *jsontree.JsonTree, 2)
		chain := func(i int, cout chan<- *jsontree.JsonTree, cin <-chan *jsontree.JsonTree) {
			j := 0
			for js := range cin {
				if js == nil {
					j++
					break
				}
				_cout := make(chan *jsontree.JsonTree)
				go path[i](_cout, js)
				for js := range _cout {
					if js == nil {
						break
					}
					cout <- js
				}
			}
			cout <- nil
		}
		for i := range path {
			if i == len(path)-1 {
				go chain(i, out, cin)
			} else {
				go chain(i, cout, cin)
				cin = cout
				cout = make(chan *jsontree.JsonTree, 2)
			}
		}
	}
}

func Count(path ...Selector) Selector {
	return func(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
		countch := make(chan *jsontree.JsonTree)
		count := 0
		go Chain(path...)(countch, js)
		for js := range countch {
			if js == nil {
				break
			}
			count++
		}
		out <- jsontree.NewNumber(float64(count))
		out <- nil
	}
}

func RecursiveDescent(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
	recDescent(out, js)
	out <- nil
}
func recDescent(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
	out <- js
	if a, err := js.Array(); err == nil {
		for i := range a {
			elem := js.GetIndex(i)
			recDescent(out, elem)
		}
	} else if m, err := js.Object(); err == nil {
		for k := range m {
			val := js.Get(k)
			recDescent(out, val)
		}
	}
}

func Identity(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
	out <- js
	out <- nil
}

func Parent(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
	jsparent := js.Parent()
	if jsparent != nil {
		out <- jsparent
	}
	out <- nil
}

func Root(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
	out <- js.Root()
	out <- nil
}

func All(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
	if a, err := js.Array(); err == nil {
		for i := range a {
			out <- js.GetIndex(i)
		}
	} else if m, err := js.Object(); err == nil {
		for k := range m {
			out <- js.Get(k)
		}
	}
	out <- nil
}

func IgnoreErrors(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
	if js.Err() == nil {
		out <- js
	}
	out <- nil
}

func JustErrors(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
	if js.Err() != nil {
		out <- js
	}
	out <- nil
}

func Key(key string) Selector {
	return func(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
		jschild := js.Get(key)
		err := js.Err()
		if err == nil {
			out <- jschild
		}
		out <- nil
	}
}

func Index(i int) Selector {
	return func(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
		a, _ := js.Array()
		if 0 < i && i < len(a) {
			out <- js.GetIndex(i)
		}
		out <- nil
	}
}

func Has(sel ...Selector) Selector {
	return func(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
		if len(Lookup(js, sel...)) > 0 {
			out <- js
		}
		out <- nil
	}
}

func EqualString(x string, sel ...Selector) Selector {
	return func(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
		for _, jschild := range Lookup(js, sel...) {
			str, err := jschild.String()
			if err == nil && str == x {
				out <- js
				break
			}
		}
		out <- nil
	}
}

func EqualFloat64(x float64, sel ...Selector) Selector {
	return func(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
		for _, jschild := range Lookup(js, sel...) {
			f, err := jschild.Number()
			if err == nil && f == x {
				out <- js
				break
			}
		}
		out <- nil
	}
}

func EqualBool(x bool, sel ...Selector) Selector {
	return func(out chan<- *jsontree.JsonTree, js *jsontree.JsonTree) {
		for _, jschild := range Lookup(js, sel...) {
			b, err := jschild.Boolean()
			if err == nil && b == x {
				out <- js
				break
			}
		}
		out <- nil
	}
}
