// Copyright 2013, Bryan Matsuo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// jsontree.go [created: Sat, 22 Jun 2013]

// Package jsontree provides a walkable tree structure for JSON data.
package jsontree

import (
	"encoding/json"
	"errors"
	"fmt"
)

// an error that includes path information
type PathError struct {
	Path string
	Err  error
}

func newPathError(path string, v ...interface{}) *PathError {
	return &PathError{
		Path: path,
		Err:  errors.New(fmt.Sprint(v...)),
	}
}

func newPathErrorf(path, format string, v ...interface{}) *PathError {
	return &PathError{
		Path: path,
		Err:  fmt.Errorf(format, v...),
	}
}

func (err *PathError) Error() string {
	return fmt.Sprintf("%v; %s", err.Err, err.Path)
}

type JsonType uint8

const (
	Object JsonType = iota
	Array
	String
	Number
	Boolean
	Null
	Error
)

var jsonTypeStrings = []string{
	Object:  "object",
	Array:   "array",
	String:  "string",
	Number:  "number",
	Boolean: "boolean",
	Null:    "null",
	Error:   "error",
}

func (t JsonType) String() string {
	if int(t) > len(jsonTypeStrings) {
		return fmt.Sprintf("Unknown (%d)", t)
	}
	return jsonTypeStrings[t]
}

type JsonTree struct {
	typ    JsonType
	root   *JsonTree
	parent *JsonTree
	init   bool
	key    string
	index  int
	err    *error
	val    interface{}
}

func newTree(val interface{}) *JsonTree {
	return &JsonTree{
		index: -1,
		val:   val,
	}
}

// creates an empty *JsonTree. initialize the tree with json.Unmarshal().
func New() *JsonTree {
	return newTree(nil)
}

func NewNull() *JsonTree {
	tree := New()
	tree.getType()
	return tree
}

func NewString(s string) *JsonTree {
	tree := newTree(s)
	tree.getType()
	return tree
}

func NewNumber(x float64) *JsonTree {
	tree := newTree(x)
	tree.getType()
	return tree
}

func NewBoolean(b bool) *JsonTree {
	tree := newTree(b)
	tree.getType()
	return tree
}

func NewObject(o map[string]interface{}) *JsonTree {
	if o == nil {
		o = make(map[string]interface{}, 0)
	}
	tree := newTree(o)
	tree.getType()
	return tree
}

func NewArray(a []interface{}) *JsonTree {
	if a == nil {
		a = make([]interface{}, 0, 1)
	}
	tree := newTree(a)
	tree.getType()
	return tree
}

func (tree *JsonTree) Type() JsonType {
	return tree.typ
}

func (tree *JsonTree) Len() (int, error) {
	switch tree.Type() {
	case Object:
		return len(tree.val.(map[string]interface{})), nil
	case Array:
		return len(tree.val.([]interface{})), nil
	default:
		return 0, fmt.Errorf("not an array or an object (%v); %s", tree.Type, tree.path())
	}
}

// any error encountered due to non-existent keys, out of range indices, etc.
func (tree *JsonTree) Err() error {
	if tree.err != nil {
		return *tree.err
	}
	return nil
}

// the root of tree. this value can not be nil.
func (tree *JsonTree) Root() *JsonTree {
	if tree.root == nil {
		return tree
	}
	return tree.root
}

// tree's parent. this value can be nil
func (tree *JsonTree) Parent() *JsonTree {
	return tree.parent
}

// a *JsonTree representing the i-th element in the array tree. if tree.Err()
// is not nil then the Err() method of returned *JsonTree returns the same error.
// if tree is not an array then the Err() method of the returned *JsonTree's
// returns a *PathError.
func (tree *JsonTree) GetIndex(i int) *JsonTree {
	child := New()
	defer child.getType()
	child.index = i
	if tree.root == nil {
		child.root = tree
	} else {
		child.root = tree.root
	}
	child.parent = tree
	child.err = tree.err
	if child.err != nil {
		return child
	}
	switch {
	case !tree.init:
		child.errUninitialized()
	case tree.typ == Array:
		a := tree.val.([]interface{})
		if 0 <= i && i < len(a) {
			child.val = a[i]
		} else {
			child.errIndexOutOfRange()
		}
	default:
		child.errTypeError(Array)
	}
	return child
}

// a *JsonTree representing the value of key in the object tree. if tree.Err()
// is not nil then the Err() method of returned *JsonTree returns the same error.
// if tree is not an object then the Err() method of the returned *JsonTree's
// returns a *PathError.
func (tree *JsonTree) Get(key string) *JsonTree {
	child := New()
	defer child.getType()
	child.key = key
	if tree.root == nil {
		child.root = tree
	} else {
		child.root = tree.root
	}
	child.parent = tree
	child.err = tree.err
	if child.err != nil {
		return child
	}
	switch {
	case !tree.init:
		child.errUninitialized()
	case tree.typ == Object:
		val, ok := tree.val.(map[string]interface{})[key]
		if ok {
			child.val = val
		} else {
			child.errNoExist()
		}
	default:
		child.errTypeError(Object)
	}
	return child
}

func (tree *JsonTree) Interface() (interface{}, error) {
	return tree.val, tree.Err()
}

// converts tree to a string. returns a *PathError if tree is not a string.
func (tree *JsonTree) String() (string, error) {
	if !tree.init {
		return "", newPathErrorf(tree.path(), "uninitialized")
	}
	switch tree.typ {
	case Error:
		return "", *tree.err
	case String:
		return tree.val.(string), nil
	default:
		return "", newPathErrorf(tree.path(), "not a string (%v)", tree.Type)
	}
}

// converts tree to a number. returns a *PathError if tree is not a number.
func (tree *JsonTree) Number() (float64, error) {
	if !tree.init {
		return 0, newPathErrorf(tree.path(), "uninitialized")
	}
	switch tree.typ {
	case Error:
		return 0, *tree.err
	case Number:
		return tree.val.(float64), nil
	default:
		return 0, newPathErrorf(tree.path(), "not a number (%v)", tree.Type)
	}
}

// converts tree to a bool. returns a *PathError if tree is not a boolean.
func (tree *JsonTree) Boolean() (bool, error) {
	if !tree.init {
		return false, newPathErrorf(tree.path(), "uninitialized")
	}
	switch tree.typ {
	case Error:
		return false, *tree.err
	case Boolean:
		return tree.val.(bool), nil
	default:
		return false, newPathErrorf(tree.path(), "not a bool (%v)", tree.Type)
	}
}

// converts tree to a slice. returns a *PathError if tree is not an array.
func (tree *JsonTree) Array() ([]interface{}, error) {
	if !tree.init {
		return nil, newPathErrorf(tree.path(), "uninitialized")
	}
	switch tree.typ {
	case Error:
		return nil, *tree.err
	case Array:
		return tree.val.([]interface{}), nil
	default:
		return nil, newPathErrorf(tree.path(), "not an array (%v)", tree.Type)
	}
}

// converts tree to a map. returns a *PathError if tree is not an object.
func (tree *JsonTree) Object() (map[string]interface{}, error) {
	if !tree.init {
		return nil, newPathErrorf(tree.path(), "uninitialized")
	}
	switch tree.typ {
	case Error:
		return nil, *tree.err
	case Object:
		return tree.val.(map[string]interface{}), nil
	default:
		return nil, newPathErrorf(tree.path(), "not an object (%v)", tree.Type)
	}
}

// returns true if tree is null. returns false in otherwise
// (other type, error, non existing keys, ...).
func (tree *JsonTree) IsNull() bool {
	return tree.typ == Null
}

// implements json.Unmarshaler
func (tree *JsonTree) UnmarshalJSON(p []byte) error {
	defer tree.getType()
	return json.Unmarshal(p, &tree.val)
}

// implements json.Marshaler
func (tree *JsonTree) MarshalJSON() ([]byte, error) {
	return json.Marshal(tree.val)
}

func (tree *JsonTree) newError(v ...interface{}) {
	err := error(newPathError(tree.path(), v...))
	tree.err = &err
}

func (tree *JsonTree) newErrorf(format string, v ...interface{}) {
	err := error(newPathErrorf(tree.path(), format, v...))
	tree.err = &err
}

func (tree *JsonTree) errUninitialized() {
	tree.newError("uninitialized")
}
func (tree *JsonTree) errNoExist() {
	tree.newError("key does not exist")
}
func (tree *JsonTree) errIndexOutOfRange() {
	tree.newErrorf("index out of range")
}
func (tree *JsonTree) errTypeError(expected JsonType) {
	if expected == Object && expected == Array {
		tree.newErrorf("not an %v (%v)", expected, tree.Type)
	} else {
		tree.newErrorf("not a %v (%v)", expected, tree.Type)
	}
}

func (tree *JsonTree) path() string {
	if tree.parent == nil {
		return "$"
	}
	pre := tree.parent.path()
	if tree.index >= 0 {
		return fmt.Sprintf("%s[%d]", pre, tree.index)
	} else {
		return fmt.Sprintf("%s.%s", pre, tree.key)
	}
}

func (tree *JsonTree) getType() {
	tree.init = true
	if tree.err != nil {
		tree.typ = Error
		return
	}
	switch tree.val.(type) {
	case string:
		tree.typ = String
	case float64:
		tree.typ = Number
	case bool:
		tree.typ = Boolean
	case nil:
		tree.typ = Null
	case []interface{}:
		tree.typ = Array
	case map[string]interface{}:
		tree.typ = Object
	}
}
