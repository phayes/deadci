// Copyright 2013, Bryan Matsuo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// parser.go [created: Fri, 21 Jun 2013]

package jsonpath

import (
	"errors"
	"fmt"
	"os"

	"github.com/bmatsuo/go-jsontree/exp/jsonpath/lexer"
)

var PARSE_DEBUG = false

func debug(v ...interface{}) {
	if PARSE_DEBUG {
		fmt.Fprint(os.Stderr, v...)
	}
}

func debugln(v ...interface{}) {
	if PARSE_DEBUG {
		fmt.Fprintln(os.Stderr, v...)
	}
}

func debugf(format string, v ...interface{}) {
	if PARSE_DEBUG {
		fmt.Fprintf(os.Stderr, format, v...)
	}
}

func Parse(input string) (Selector, error) {
	selectors := make([]Selector, 0, 1)
	lex := lexer.New(input)
	for {
		switch item := lex.Next(); item.Type {
		case lexer.ItemEOF:
			debug("EOF\n")
			debugf("%d selectors\n", len(selectors))
			switch len(selectors) {
			case 0:
				return nil, fmt.Errorf("empty")
			case 1:
				return selectors[0], nil
			default:
				return Chain(selectors...), nil
			}
		case lexer.ItemError:
			debug("ERROR\n")
			return nil, errors.New(item.Value)
		case lexer.ItemDollar:
			debug("DOLLAR ")
			next := lex.Next()
			if next.Type != lexer.ItemDot {
				return nil, fmt.Errorf("expected \".\" but got %q", next.Value)
			}
			fallthrough
		case lexer.ItemDotDot:
			debug("DOTDOT ")
			fallthrough // FIXME
		case lexer.ItemDot:
			debug("DOT\n")
			switch next := lex.Next(); next.Type {
			case lexer.ItemEOF:
				return nil, errors.New("unexpected EOF")
			case lexer.ItemStarStar:
				debug("STAR STAR\n")
				selectors = append(selectors, RecursiveDescent)
			case lexer.ItemStar:
				debug("STAR\n")
				selectors = append(selectors, All)
			case lexer.ItemPathKey:
				debug("PATH KEY %s\n", next.Value)
				selectors = append(selectors, Key(next.Value))
			default:
				return nil, fmt.Errorf("expected key but got %q", next.Value)
			}
		case lexer.ItemLeftBracket:
			debug("LEFTBRACKET\n")
			sel, err := parseBracket(lex)
			if err != nil {
				return nil, err
			}
			selectors = append(selectors, sel)
		}
	}
}

func parseBracket(lex lexer.Interface) (Selector, error) {
	return nil, fmt.Errorf("not implemented")
}
