// Copyright 2013 Google Inc. All Rights Reserved.
//
// 	Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file implements a simplified sexp parser, suitable for reading
// package definitions.

package elpa

import (
	"strings"
)

const (
	OPEN_PAREN = iota
	CLOSE_PAREN
	SYMBOL
	STRING
	EOF
)

type Token struct {
	Type      int
	StringVal string
}

var packageDefinitionString string = "define-package"

func parseSimpleSexp(cin chan int, cout chan *Token, cquit chan bool) {
	parenCount := 0
	endActions := map[int]func(){}
	for {
		var b int
		select {
		case b = <-cin:
			// Proceed...
		case <-cquit:
			// Got quit signal, return.
			cout <- &Token{Type: EOF}
			return
		}
		switch {
		case b == '\'':
			cout <- &Token{Type: OPEN_PAREN}
			cout <- &Token{Type: SYMBOL, StringVal: "quote"}
			endActions[parenCount] = func() { cout <- &Token{Type: CLOSE_PAREN} }
			parenCount++
		case b == '(':
			cout <- &Token{Type: OPEN_PAREN}
			parenCount++
		case b == ')':
			cout <- &Token{Type: CLOSE_PAREN}
			if action, ok := endActions[parenCount]; ok {
				action()
				delete(endActions, parenCount)
			}
			parenCount--
		case (b >= 'A' && b <= 'Z') ||
			(b >= 'a' && b <= 'z') ||
			b == '-':
			sym := make([]byte, 1)
			sym[0] = byte(b)
			for {
				b = <-cin
				if b == ' ' || b == '\n' {
					// We've reached the end of the symbol
					break
				}
				sym = append(sym, byte(b))
			}
			cout <- &Token{Type: SYMBOL, StringVal: strings.ToLower(string(sym))}
		case b == '"':
			s := make([]byte, 0)
			for {
				b = <-cin
				if b == '"' {
					// We've reached the end of the string
					break
				}
				if b == '\\' {
					b = <-cin
				}
				s = append(s, byte(b))
			}
			cout <- &Token{Type: STRING, StringVal: string(s)}
		}
	}
}
