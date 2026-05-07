package main

import (
	"fmt"
	"strings"
)

func splitDetail(detail string) []string {
	if detail == "" {
		return nil
	}
	parts := strings.Split(detail, ",")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			lines = append(lines, part)
		}
	}
	return lines
}

func typeDescription(signature string) string {
	if signature == "" {
		return "unknown"
	}
	parser := signatureParser{input: []rune(signature)}
	parts := make([]string, 0)
	for parser.pos < len(parser.input) {
		parts = append(parts, parser.parseType())
	}
	return strings.Join(parts, ", ")
}

type signatureParser struct {
	input []rune
	pos   int
}

func (p *signatureParser) parseType() string {
	if p.pos >= len(p.input) {
		return "unknown"
	}
	ch := p.input[p.pos]
	p.pos++

	switch ch {
	case 'y':
		return "byte"
	case 'b':
		return "boolean"
	case 'n':
		return "int16"
	case 'q':
		return "uint16"
	case 'i':
		return "int32"
	case 'u':
		return "uint32"
	case 'x':
		return "int64"
	case 't':
		return "uint64"
	case 'd':
		return "double"
	case 'h':
		return "unix fd"
	case 's':
		return "string"
	case 'o':
		return "object path"
	case 'g':
		return "signature"
	case 'v':
		return "variant"
	case 'a':
		if p.pos < len(p.input) && p.input[p.pos] == '{' {
			p.pos++
			key := p.parseType()
			value := p.parseType()
			if p.pos < len(p.input) && p.input[p.pos] == '}' {
				p.pos++
			}
			return fmt.Sprintf("dictionary<%s, %s>", key, value)
		}
		return "array<" + p.parseType() + ">"
	case '(':
		items := make([]string, 0)
		for p.pos < len(p.input) && p.input[p.pos] != ')' {
			items = append(items, p.parseType())
		}
		if p.pos < len(p.input) && p.input[p.pos] == ')' {
			p.pos++
		}
		return "struct<" + strings.Join(items, ", ") + ">"
	case '{':
		key := p.parseType()
		value := p.parseType()
		if p.pos < len(p.input) && p.input[p.pos] == '}' {
			p.pos++
		}
		return fmt.Sprintf("dict entry<%s, %s>", key, value)
	default:
		return fmt.Sprintf("unknown(%c)", ch)
	}
}
