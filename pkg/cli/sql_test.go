// Copyright 2016 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License included
// in the file licenses/BSL.txt and at www.mariadb.com/bsl11.
//
// Change Date: 2022-10-01
//
// On the date above, in accordance with the Business Source License, use
// of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt and at
// https://www.apache.org/licenses/LICENSE-2.0

package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
)

// Example_sql_lex tests the usage of the lexer in the sql subcommand.
func Example_sql_lex() {
	c := newCLITest(cliTestParams{insecure: true})
	defer c.cleanup()

	conn := makeSQLConn(fmt.Sprintf("postgres://%s@%s/?sslmode=disable",
		security.RootUser, c.ServingAddr()))
	defer conn.Close()

	tests := []string{`
select '
\?
;
';
`,
		`
select ''''
;

select '''
;
''';
`,
		`select 1 as "1";
-- just a comment without final semicolon`,
	}

	setCLIDefaultsForTests()

	// We need a temporary file with a name guaranteed to be available.
	// So open a dummy file.
	f, err := ioutil.TempFile("", "input")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return
	}
	// Get the name and close it.
	fname := f.Name()
	f.Close()

	// At every point below, when t.Fatal is called we should ensure the
	// file is closed and removed.
	f = nil
	defer func() {
		if f != nil {
			f.Close()
		}
		_ = os.Remove(fname)
		stdin = os.Stdin
	}()

	for _, test := range tests {
		// Populate the test input.
		if f, err = os.OpenFile(fname, os.O_WRONLY, 0666); err != nil {
			fmt.Fprintln(stderr, err)
			return
		}
		if _, err := f.WriteString(test); err != nil {
			fmt.Fprintln(stderr, err)
			return
		}
		f.Close()
		// Make it available for reading.
		if f, err = os.Open(fname); err != nil {
			fmt.Fprintln(stderr, err)
			return
		}
		// Override the standard input for runInteractive().
		stdin = f

		redirectOutput(func() {
			err := runInteractive(conn)
			if err != nil {
				fmt.Fprintln(stderr, err)
			}
		})
	}

	// Output:
	// ?column?
	// +----------+
	//
	//   \?
	//   ;
	//
	// (1 row)
	//   ?column?
	// +----------+
	//   '
	// (1 row)
	//   ?column?
	// +----------+
	//   '
	//   ;
	//   '
	// (1 row)
	//   1
	// +---+
	//   1
	// (1 row)
}

func TestIsEndOfStatement(t *testing.T) {
	defer leaktest.AfterTest(t)()

	tests := []struct {
		in         string
		isEnd      bool
		isNotEmpty bool
	}{
		{
			in:         ";",
			isEnd:      true,
			isNotEmpty: true,
		},
		{
			in:         "; /* comment */",
			isEnd:      true,
			isNotEmpty: true,
		},
		{
			in:         "; SELECT",
			isNotEmpty: true,
		},
		{
			in:         "SELECT",
			isNotEmpty: true,
		},
		{
			in:         "SET; SELECT 1;",
			isEnd:      true,
			isNotEmpty: true,
		},
		{
			in:         "SELECT ''''; SET;",
			isEnd:      true,
			isNotEmpty: true,
		},
		{
			in: "  -- hello",
		},
		{
			in:         "select 'abc", // invalid token
			isNotEmpty: true,
		},
		{
			in:         "'abc", // invalid token
			isNotEmpty: true,
		},
		{
			in:         `SELECT e'\xaa';`, // invalid token but last token is semicolon
			isEnd:      true,
			isNotEmpty: true,
		},
	}

	for _, test := range tests {
		lastTok, isNotEmpty := parser.LastLexicalToken(test.in)
		if isNotEmpty != test.isNotEmpty {
			t.Errorf("%q: isNotEmpty expected %v, got %v", test.in, test.isNotEmpty, isNotEmpty)
		}
		isEnd := isEndOfStatement(lastTok)
		if isEnd != test.isEnd {
			t.Errorf("%q: isEnd expected %v, got %v", test.in, test.isEnd, isEnd)
		}
	}
}
