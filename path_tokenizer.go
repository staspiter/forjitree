package forjitree

import (
	"encoding/csv"
	"regexp"
	"strings"
)

const (
	PathTokenKindThis = iota
	PathTokenKindParent
	PathTokenKindRoot
	PathTokenKindSub
	PathTokenKindParams
	PathTokenKindDirectChildren
	PathTokenKindAllChildren
	PathTokenKindAllParents
)

const (
	ParamTypeEquals = iota
	ParamTypePresence
	ParamTypeNotEquals
	ParamTypeNotPresence
	ParamTypeGreaterThan
	ParamTypeLessThan
	ParamTypeGreaterOrEquals
	ParamTypeLessOrEquals
	ParamTypeRegex
)

type pathTokenParam struct {
	Key        string
	Value      string
	ParamType  int
	ValueRegex *regexp.Regexp
}

type pathToken struct {
	Kind   int
	Key    string
	Params []pathTokenParam
}

func splitCSV(s string, delimeter rune) []string {
	r := csv.NewReader(strings.NewReader(s))
	r.Comma = delimeter
	record, err := r.Read()
	if err != nil {
		return nil
	}
	return record
}

func TokenizePath(path string) []pathToken {
	const subToken rune = '/'
	const paramsToken rune = '['
	const paramsCloseToken rune = ']'
	const escapeToken rune = '\\'

	var paramsTokenCounter = 0
	var tokensStr []string
	t := ""
	for i := 0; i < len(path); i++ {

		// Escape token
		if rune(path[i]) == escapeToken {
			i++
			if i >= len(path) {
				break
			}
			t += string(path[i])
			continue
		}

		if rune(path[i]) == subToken && paramsTokenCounter == 0 {
			tokensStr = append(tokensStr, t)
			t = ""

		} else if rune(path[i]) == paramsToken {
			if paramsTokenCounter == 0 {
				tokensStr = append(tokensStr, t)
				t = ""
			}
			paramsTokenCounter++

		} else if rune(path[i]) == paramsCloseToken {
			paramsTokenCounter--
		}

		t += string(path[i])
	}
	tokensStr = append(tokensStr, t)

	var tokens []pathToken
	for i, ts := range tokensStr {
		var t pathToken
		t.Kind = PathTokenKindThis

		if ts == "" && i == 0 && len(tokensStr) > 1 && strings.HasPrefix(tokensStr[1], "/") {
			t.Kind = PathTokenKindRoot

		} else if (ts == ".." && i == 0) || ts == "/.." {
			t.Kind = PathTokenKindParent

		} else if (ts == "..." && i == 0) || ts == "/..." {
			t.Kind = PathTokenKindAllParents

		} else if (ts == "*" && i == 0) || ts == "/*" {
			t.Kind = PathTokenKindDirectChildren

		} else if (ts == "**" && i == 0) || ts == "/**" {
			t.Kind = PathTokenKindAllChildren

		} else if strings.HasPrefix(ts, "[") && strings.HasSuffix(ts, "]") {
			// [key=value,key] filter token
			t.Kind = PathTokenKindParams
			pairs := splitCSV(strings.Trim(ts, "[]"), ',')
			for _, p := range pairs {

				if strings.Contains(p, "=") {
					// Value equation
					dividerPos := strings.Index(p, "=")
					t.Params = append(t.Params, pathTokenParam{Key: p[:dividerPos], Value: p[dividerPos+1:], ParamType: ParamTypeEquals})

				} else if strings.Contains(p, "!=") {
					// Value not equals
					dividerPos := strings.Index(p, "!=")
					t.Params = append(t.Params, pathTokenParam{Key: p[:dividerPos], Value: p[dividerPos+2:], ParamType: ParamTypeNotEquals})

				} else if strings.Contains(p, ">") {
					// Value greater than
					dividerPos := strings.Index(p, ">")
					t.Params = append(t.Params, pathTokenParam{Key: p[:dividerPos], Value: p[dividerPos+1:], ParamType: ParamTypeGreaterThan})

				} else if strings.Contains(p, "<") {
					// Value less than
					dividerPos := strings.Index(p, "<")
					t.Params = append(t.Params, pathTokenParam{Key: p[:dividerPos], Value: p[dividerPos+1:], ParamType: ParamTypeLessThan})

				} else if strings.Contains(p, ">=") {
					// Value greater or equals
					dividerPos := strings.Index(p, ">=")
					t.Params = append(t.Params, pathTokenParam{Key: p[:dividerPos], Value: p[dividerPos+2:], ParamType: ParamTypeGreaterOrEquals})

				} else if strings.Contains(p, "<=") {
					// Value less or equals
					dividerPos := strings.Index(p, "<=")
					t.Params = append(t.Params, pathTokenParam{Key: p[:dividerPos], Value: p[dividerPos+2:], ParamType: ParamTypeLessOrEquals})

				} else if strings.Contains(p, "~") {
					// Value regex
					dividerPos := strings.Index(p, "~")
					valueRegex, err := regexp.Compile(p[dividerPos+1:])
					if err != nil {
						t.Params = append(t.Params, pathTokenParam{Key: p[:dividerPos], ValueRegex: valueRegex, ParamType: ParamTypeRegex})
					}

				} else if strings.HasPrefix(p, "!") {
					// Check for key absence
					t.Params = append(t.Params, pathTokenParam{Key: p[1:], Value: "", ParamType: ParamTypeNotPresence})

				} else {
					// Check for key presense
					t.Params = append(t.Params, pathTokenParam{Key: p, Value: "", ParamType: ParamTypePresence})
				}

			}

		} else if strings.HasPrefix(ts, "/") {
			if len(ts) > 1 {
				t.Kind = PathTokenKindSub
				t.Key = strings.TrimPrefix(ts, "/")
			} else {
				t.Kind = PathTokenKindThis
			}

		} else if i == 0 && ts != "" {
			t.Kind = PathTokenKindSub
			t.Key = ts
		}

		tokens = append(tokens, t)
	}

	return tokens
}
