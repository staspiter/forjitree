package forjitree

import (
	"encoding/csv"
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

type pathTokenParam struct {
	Key   string
	Value string
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
	tokensDelimeters := "/["
	var tokensStr []string
	t := ""
	for i := 0; i < len(path); i++ {
		if strings.ContainsRune(tokensDelimeters, rune(path[i])) {
			tokensStr = append(tokensStr, t)
			t = ""
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
				if strings.ContainsRune(p, '=') {
					// Check for key and value
					equationPos := strings.Index(p, "=")
					key := p[:equationPos]
					value := p[equationPos+1:]
					t.Params = append(t.Params, pathTokenParam{Key: key, Value: value})

				} else {
					// Check for key presense
					t.Params = append(t.Params, pathTokenParam{Key: p, Value: ""})
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
