package recall

import (
	"strings"
	"unicode"
)

const (
	opAND = "AND"
	opOR  = "OR"
	opNOT = "NOT"
)

type queryToken struct {
	text     string
	operator string
	prefix   bool
}

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

func searchable(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	})
}

func phrase(token queryToken) string {
	out := `"` + strings.ReplaceAll(token.text, `"`, `""`) + `"`
	if token.prefix {
		out += "*"
	}
	return out
}

func buildMatch(query string) (string, error) {
	tokens := lexQuery(query)
	var terms []queryToken
	for _, token := range tokens {
		if token.operator == "" && searchable(token.text) {
			terms = append(terms, token)
		}
	}
	if len(terms) == 0 {

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
