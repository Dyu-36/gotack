package recall

import (
	"strings"
	"unicode"
)

// FTS5 query surface for the keyword shape of session_search.
//
// Hermes accepts the tokenizer's own syntax: several words mean AND, a
// double-quoted run is an exact phrase, OR and NOT combine clauses, and a
// trailing star matches by prefix. gotack used to wrap every token in
// quotes, which silently turned `deploy OR rollback` into a hunt for three
// literal words.
//
// Handing raw user text to MATCH is not an option either: an unbalanced
// quote, a column filter such as `title:x`, or a stray `NEAR/` makes SQLite
// raise a syntax error, and a tool call must never fail on punctuation. So
// the expression is re-emitted from a parse. Every term becomes a quoted
// phrase (optionally carrying the prefix star) and only the three operator
// keywords survive unquoted. Whatever the parse cannot shape into a valid
// infix expression falls back to literal-phrase mode so malformed input
// remains searchable.
const (
	opAND = "AND"
	opOR  = "OR"
	opNOT = "NOT"
)

// queryToken is one lexed unit: either an operator or a phrase term.
type queryToken struct {
	text     string // phrase text, quotes already stripped
	operator string // opAND, opOR or opNOT when this token is an operator
	prefix   bool   // the term carried a trailing star
}

// lexQuery splits a query into operators, quoted phrases and bare terms. An
// unterminated quote is tolerated: the remainder of the input becomes the
// last phrase, so a half-typed query still searches.
func lexQuery(query string) []queryToken {
	var tokens []queryToken
	runes := []rune(query)
	for i := 0; i < len(runes); {
		switch r := runes[i]; {
		case unicode.IsSpace(r):
			i++
		case r == '"':
			i++
			var phraseText strings.Builder
			for i < len(runes) && runes[i] != '"' {
				phraseText.WriteRune(runes[i])
				i++
			}
			if i < len(runes) {
				i++
			}
			prefix := false
			if i < len(runes) && runes[i] == '*' {
				prefix = true
				i++
			}
			tokens = append(tokens, queryToken{text: phraseText.String(), prefix: prefix})
		default:
			start := i
			for i < len(runes) && !unicode.IsSpace(runes[i]) && runes[i] != '"' {
				i++
			}
			word := string(runes[start:i])
			// FTS5 only treats the keywords as operators in upper case, so a
			// lower-case "and" stays a searchable word.
			if word == opAND || word == opOR || word == opNOT {
				tokens = append(tokens, queryToken{operator: word})
				continue
			}
			tokens = append(tokens, queryToken{
				text:   strings.TrimSuffix(word, "*"),
				prefix: strings.HasSuffix(word, "*"),
			})
		}
	}
	return tokens
}

// searchable reports whether text holds a letter or a digit. A term made
// only of punctuation cannot match anything and must not anchor an operator.
func searchable(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	})
}

// phrase renders one term as an FTS5 phrase, doubling embedded quotes so no
// term can close its own string and inject syntax.
func phrase(token queryToken) string {
	out := `"` + strings.ReplaceAll(token.text, `"`, `""`) + `"`
	if token.prefix {
		out += "*"
	}
	return out
}

// buildMatch converts user input into a MATCH expression, or ErrInvalidQuery
// when the input holds no searchable word at all.
func buildMatch(query string) (string, error) {
	tokens := lexQuery(query)
	var terms []queryToken
	for _, token := range tokens {
		if token.operator == "" && searchable(token.text) {
			terms = append(terms, token)
		}
	}
	if len(terms) == 0 {
		// Operator keywords with nothing to combine are not an expression;
		// treat them as the literal words they look like rather than failing
		// a call the user meant as a search for "AND".
		for _, token := range tokens {
			if token.operator != "" {
				terms = append(terms, queryToken{text: token.operator})
			}
		}
	}
	if len(terms) == 0 {
		return "", ErrInvalidQuery
	}
	if expr, ok := infixExpression(tokens); ok {
		return expr, nil
	}
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		parts = append(parts, phrase(term))
	}
	return strings.Join(parts, " "), nil
}

// infixExpression re-emits the token stream when the operators form a
// well-shaped infix expression: no leading or trailing operator and never
// two operators in a row. Adjacent phrases are left adjacent, which is FTS5's
// implicit AND and matches what Hermes documents for several words.
func infixExpression(tokens []queryToken) (string, bool) {
	var parts []string
	expectTerm := true
	for _, token := range tokens {
		if token.operator != "" {
			if expectTerm {
				return "", false
			}
			parts = append(parts, token.operator)
			expectTerm = true
			continue
		}
		if !searchable(token.text) {
			continue
		}
		parts = append(parts, phrase(token))
		expectTerm = false
	}
	if expectTerm || len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, " "), true
}
