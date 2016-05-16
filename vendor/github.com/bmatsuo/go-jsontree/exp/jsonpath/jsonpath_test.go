// Copyright 2013, Bryan Matsuo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// jsonpath_test.go [created: Mon, 10 Jun 2013]

package jsonpath

import (
	"github.com/bmatsuo/go-jsontree"
	y "github.com/bmatsuo/yup"
	yt "github.com/bmatsuo/yup/yuptype"

	"testing"
)

type TestError string

func testSel(t *testing.T, sel Selector, jsstr string, val ...interface{}) {
	out := make(chan *jsontree.JsonTree, 2)
	_js := jsontree.New()
	err := _js.UnmarshalJSON([]byte(jsstr))
	if err != nil {
		yt.Nil(t, err)
	}
	go sel(out, _js)
	i := 0
	for js := range out {
		if js == nil {
			break
		}
		if i < len(val) {
			if err := js.Err(); err != nil {
				_, ok := val[i].(TestError)
				y.Assert(t, ok, "unexpected error: ", err)
				i++
				continue
			}
			v, _ := js.Interface()
			yt.Equal(t, val[i], v, jsstr)
		}
		i++
	}
	yt.Equal(t, len(val), i, "wrong number of tests ", jsstr)
}

func TestKey(t *testing.T) {
	sel := Key("test")
	testSel(t, sel, `{"test":"abc"}`, "abc")
	testSel(t, sel, `{"tset":"cba"}`, TestError(""))
	testSel(t, sel, `"[]"`, TestError(""))
	testSel(t, sel, `"abc"`, TestError(""))
	testSel(t, sel, `"null"`, TestError(""))
}

func TestIndex(t *testing.T) {
	sel := Index(1)
	testSel(t, sel, `["abc", "def"]`, "def")
	testSel(t, sel, `[]`)
	testSel(t, sel, `{}`)
	testSel(t, sel, `null`)
}

func TestAll(t *testing.T) {
	testSel(t, All, `{"a":1, "b":2, "c":3}`, float64(1), float64(2), float64(3))
	testSel(t, All, `["a", "b", "c"]`, "a", "b", "c")
}

func TestChain(t *testing.T) {
	sel := Chain(Key("outer"), Key("inner"))
	testSel(t, sel, `{"outer":{"inner":"abc"}}`, "abc")
	testSel(t, sel, `{"outer":{"nope":"abc"}}`, TestError(""))
	testSel(t, sel, `{}`)
	testSel(t, sel, `[]`)
}

func TestHas(t *testing.T) {
	sel := Chain(All, Has(Key("sub"), Key("subinner")))
	testSel(t, sel, `{"outer":{"sub":{"subinner":1}}, "nope":"nuh uh"}`, map[string]interface{}{
		"sub": map[string]interface{}{
			"subinner": float64(1),
		},
	})
	testSel(t, sel, `[{"sub":{"subinner":1}},{"nope":"nuh uh"}]`, map[string]interface{}{
		"sub": map[string]interface{}{
			"subinner": float64(1),
		},
	})
}

func TestEqualString(t *testing.T) {
	sel := EqualString("testvalue", Key("test"))
	testSel(t, sel, `{"test":"testvalue"}`, map[string]interface{}{"test": "testvalue"})
	testSel(t, sel, `{"test":123}`)
	testSel(t, sel, `{"nope":"nuh uh"}`)
}

func TestEqualFloat64(t *testing.T) {
	sel := EqualFloat64(123, Key("test"))
	testSel(t, sel, `{"test":123}`, map[string]interface{}{"test": float64(123)})
	testSel(t, sel, `{"test":"abc"}`)
	testSel(t, sel, `{"nope":"nuh uh"}`)
}

func TestEqualBool(t *testing.T) {
	sel := EqualBool(false, Key("test"))
	testSel(t, sel, `{"test":false}`, map[string]interface{}{"test": false})
	testSel(t, sel, `{"test":"abc"}`)
	testSel(t, sel, `{"nope":"nuh uh"}`)
}

func TestParse(t *testing.T) {
	sel, err := Parse(".test")
	yt.Nil(t, err)
	testSel(t, sel, `{"test":"ok"}`, "ok")
	testSel(t, sel, `{"no":"negative"}`, TestError(""))

	sel, err = Parse(".test.nesting")
	yt.Nil(t, err)
	testSel(t, sel, `{"test":{"nesting":"good"}}`, "good")
	testSel(t, sel, `{"test":{"bad":"miss"}}`, TestError(""))
	testSel(t, sel, `{"test":"miss"}`, TestError(""))
	testSel(t, sel, `{"something":{"nesting":"miss"}}`)

	sel, err = Parse(".test.*")
	yt.Nil(t, err)
	testSel(t, sel, `{"test":{"foo1":{"bar":true}, "foo2":{"bar":true}},"bar":{"foo":false}}`,
		map[string]interface{}{"bar": true},
		map[string]interface{}{"bar": true})
	sel, err = Parse(".test.*.bar")
	yt.Nil(t, err)
	testSel(t, sel, `{"test":{"foo1":{"bar":true}, "foo2":{"bar":true}},"bar":{"foo":false}}`,
		true, true)
	testSel(t, sel, `{"test":{"foo1":{"bar":true}, "foo2":"bar"},"bar":{"foo":false}}`,
		true, TestError(""))

	sel, err = Parse(".test.**")
	yt.Nil(t, err)
	testSel(t, sel, `{"test":{"foo1":{"bar":true}, "foo2":{"bar":true}},"bar":{"foo":false}}`,
		map[string]interface{}{
			"foo1": map[string]interface{}{"bar": true},
			"foo2": map[string]interface{}{"bar": true},
		},
		map[string]interface{}{"bar": true},
		true,
		map[string]interface{}{"bar": true},
		true)
	sel, err = Parse(".test.**.qux")
	yt.Nil(t, err)
	//testSel(t, sel, `{"test":{"foo1":{"bar":{"qux":true}}, "foo2":{"bar":{"qux":true}}},"bar":{"qux":true}}`,
	//	true, true, true)
}
