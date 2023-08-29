package forjitree

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"unicode"
)

var IgnoreColumn ignoreColumn

type ignoreColumn struct{}

var NilReflectValue = reflect.Zero(reflect.TypeOf((*error)(nil)).Elem())

func (ignoreColumn) Scan(value interface{}) error {
	return nil
}

func MakePatchWithPath(path string, object any, resolveEqualsSign bool) any {
	pathArr := strings.Split(path, "/")
	if len(pathArr) == 0 || (len(pathArr) == 1 && pathArr[0] == "") {
		return object
	}
	m := map[string]any{}
	m1 := m
	for i := 0; i < len(pathArr); i++ {
		p := pathArr[i]

		skipSubpath := false
		if resolveEqualsSign && strings.ContainsRune(p, '=') {
			pSplit := strings.Split(p, "=")
			m1[pSplit[0]] = pSplit[1]
			skipSubpath = true
		}

		if i == len(pathArr)-1 {
			if skipSubpath {
				// Need to merge the object with the existing one
				switch ot := object.(type) {
				case map[string]any:
					for k, v := range ot {
						m1[k] = v
					}
				case []any:
					m1[p] = ot
				}
			} else {
				m1[p] = object
			}
		} else {
			if !skipSubpath {
				m1[p] = map[string]any{}
				m1 = m1[p].(map[string]any)
			}
		}
	}
	return m
}

func WaitForInterruption() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func RandString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func CloneArray(m []any) []any {
	cp := make([]any, len(m))
	for i, v := range m {
		switch vt := v.(type) {
		case map[string]any:
			cp[i] = CloneMap(vt)
		case []any:
			cp[i] = CloneArray(vt)
		default:
			cp[i] = v
		}
	}
	return cp
}

func CloneMap(m map[string]any) map[string]any {
	cp := make(map[string]any)
	for k, v := range m {
		switch vt := v.(type) {
		case map[string]any:
			cp[k] = CloneMap(vt)
		case []any:
			cp[k] = CloneArray(vt)
		default:
			cp[k] = v
		}
	}
	return cp
}

func GetValueFromArray(m []any, path []string) any {
	if len(path) == 0 {
		return m
	}
	if i, err := strconv.Atoi(path[0]); err == nil {
		if i < len(m) {
			if len(path) == 1 {
				return m[i]
			}
			if vm, ok := m[i].(map[string]any); ok {
				return GetValueFromMap(vm, path[1:])
			} else if vArr, ok := m[i].([]any); ok {
				return GetValueFromArray(vArr, path[1:])
			}
		}
	}
	return nil
}

func GetValueFromMap(m map[string]any, path []string) any {
	if len(path) == 0 {
		return m
	}
	if v, ok := m[path[0]]; ok {
		if len(path) == 1 {
			return v
		}
		if vm, ok := v.(map[string]any); ok {
			return GetValueFromMap(vm, path[1:])
		} else if vArr, ok := v.([]any); ok {
			return GetValueFromArray(vArr, path[1:])
		}
	}
	return nil
}

func SubstituteValuesInArrayUsingFunction(m []any, f func(string, []string) any, path []string) {
	for i := 0; i < len(m); i++ {
		if vm, ok := m[i].(map[string]any); ok {
			SubstituteValuesInMapUsingFunction(vm, f, append(path, strconv.Itoa(i)))
		} else if vArr, ok := m[i].([]any); ok {
			SubstituteValuesInArrayUsingFunction(vArr, f, append(path, strconv.Itoa(i)))
		} else if vStr, ok := m[i].(string); ok {
			m[i] = f(vStr, append(path, strconv.Itoa(i)))
		}
	}
}

func SubstituteValuesInMapUsingFunction(m map[string]any, f func(string, []string) any, path []string) {
	for k, v := range m {
		if vm, ok := v.(map[string]any); ok {
			SubstituteValuesInMapUsingFunction(vm, f, append(path, k))
		} else if vArr, ok := v.([]any); ok {
			SubstituteValuesInArrayUsingFunction(vArr, f, append(path, k))
		} else if vStr, ok := v.(string); ok {
			m[k] = f(vStr, append(path, k))
		}
	}
}

func SubstituteValuesInArray(m []any, substitutes map[string]any) {
	SubstituteValuesInArrayUsingFunction(m, func(s string, path []string) any {
		if strings.HasPrefix(s, ":") {
			if substitute, ok := substitutes[strings.TrimPrefix(s, ":")]; ok {
				return substitute
			}
		}
		return s
	}, nil)
}

func SubstituteValuesInMap(m map[string]any, substitutes map[string]any) {
	SubstituteValuesInMapUsingFunction(m, func(s string, path []string) any {
		if strings.HasPrefix(s, ":") {
			if substitute, ok := substitutes[strings.TrimPrefix(s, ":")]; ok {
				return substitute
			}
		}
		return s
	}, nil)
}

func Capitalize(str string) string {
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func ExtractTrunkPathAndValue(obj map[string]any) (string, any) {
	var p any
	p = obj
	path := []string{}
	var value any = nil
	for {
		if pMap, pIsMap := p.(map[string]any); pIsMap {
			if len(pMap) == 1 {
				keys := make([]string, len(pMap))
				for k := range pMap {
					keys = append(keys, k)
				}
				path = append(path, keys[0])
				p = pMap[keys[0]]
			} else {
				value = p
				break
			}
		} else if pMap, pIsMap := p.(map[string]string); pIsMap {
			if len(pMap) == 1 {
				keys := make([]string, len(pMap))
				for k := range pMap {
					keys = append(keys, k)
				}
				path = append(path, keys[0])
				p = pMap[keys[0]]
			} else {
				value = p
				break
			}
		} else {
			value = p
			break
		}
	}
	return strings.Join(path, "/"), value
}
