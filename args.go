package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
)

const argPlaceholder = "$ARG$"

// argMarker stands in for the n-th $ARG$ while the command template is split
// into words. It is purely alphanumeric so shellquote treats it like any
// other word character, whatever quoting surrounds it.
func argMarker(n int) string {
	return "GORPExARGx" + strconv.Itoa(n) + "xEND"
}

// buildArgv turns a command template and the request arguments into an argv.
//
// The template is split into words *before* the arguments are inserted, so an
// argument can never change the template's word boundaries or escape its
// quoting:
//   - an unquoted $ARG$ that is a whole word is replaced by the argument split
//     with shell quoting rules (so "-u foo -n 100" or `--since "1 hour ago"`
//     become several words, as the *_wild commands expect);
//   - a quoted $ARG$ ('$ARG$', "$ARG$") or one embedded in a larger word
//     ($ARG$%, -w$ARG$) is replaced by the argument verbatim inside that one
//     word, quotes and all.
//
// Placeholders are filled in order; extra arguments are ignored.
func buildArgv(template string, args []string) ([]string, error) {
	if strings.Contains(template, "GORPExARGx") {
		return nil, fmt.Errorf("command template contains reserved marker GORPExARGx")
	}

	// Pass 1: replace each $ARG$ with a marker, remembering whether it sits
	// inside single or double quotes.
	var b strings.Builder
	var quoted []bool
	inSingle, inDouble := false, false
	for i := 0; i < len(template); {
		if strings.HasPrefix(template[i:], argPlaceholder) {
			b.WriteString(argMarker(len(quoted)))
			quoted = append(quoted, inSingle || inDouble)
			i += len(argPlaceholder)
			continue
		}
		c := template[i]
		switch {
		case c == '\\' && !inSingle && i+1 < len(template):
			b.WriteString(template[i : i+2])
			i += 2
			continue
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		}
		b.WriteByte(c)
		i++
	}
	if len(quoted) > len(args) {
		return nil, fmt.Errorf("not enough command arguments! Expected %d and found %d", len(quoted), len(args))
	}

	words, err := shellquote.Split(b.String())
	if err != nil {
		return nil, fmt.Errorf("could not parse command template: %w", err)
	}

	// Pass 2: substitute per word.
	argv := make([]string, 0, len(words))
	for _, word := range words {
		if n, ok := wholeMarker(word, len(quoted)); ok && !quoted[n] {
			split, err := shellquote.Split(args[n])
			if err != nil {
				return nil, fmt.Errorf("could not parse command argument %d: %w", n+1, err)
			}
			argv = append(argv, split...)
			continue
		}
		for n := range quoted {
			word = strings.ReplaceAll(word, argMarker(n), args[n])
		}
		argv = append(argv, word)
	}
	return argv, nil
}

// wholeMarker reports whether word is exactly the marker of placeholder n.
func wholeMarker(word string, count int) (int, bool) {
	for n := 0; n < count; n++ {
		if word == argMarker(n) {
			return n, true
		}
	}
	return 0, false
}
