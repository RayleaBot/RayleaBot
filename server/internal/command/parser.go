package command

import "strings"

type Parser struct {
	prefixes []string // sorted by length descending for longest-prefix-first matching
}

type ParseResult struct {
	IsCommand bool
	Command   string
	Args      []string
	Prefix    string
}

func NewParser(prefixes []string) *Parser {
	// Copy and sort by length descending so longest prefix matches first.
	sorted := make([]string, len(prefixes))
	copy(sorted, prefixes)
	// Simple insertion sort is fine for small arrays.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && len(sorted[j]) > len(sorted[j-1]); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return &Parser{prefixes: sorted}
}

func (p *Parser) Parse(plainText string) ParseResult {
	if p == nil || len(p.prefixes) == 0 {
		return ParseResult{}
	}

	text := strings.TrimSpace(plainText)
	if text == "" {
		return ParseResult{}
	}

	for _, prefix := range p.prefixes {
		if !strings.HasPrefix(text, prefix) {
			continue
		}
		rest := strings.TrimSpace(text[len(prefix):])
		if rest == "" {
			// Just the prefix alone, no command name
			return ParseResult{}
		}

		parts := strings.Fields(rest)
		return ParseResult{
			IsCommand: true,
			Command:   parts[0],
			Args:      parts[1:],
			Prefix:    prefix,
		}
	}

	return ParseResult{}
}
