package forjitree

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unicode"
)

var FunctionRegex = regexp.MustCompile(`^([a-zA-Z]+)\((.*)\)$`)

var IgnoreColumn ignoreColumn

type ignoreColumn struct{}

var NilReflectValue = reflect.Zero(reflect.TypeOf((*error)(nil)).Elem())

func (ignoreColumn) Scan(value interface{}) error {
	return nil
}

func MakePatchWithPath(path string, object any, resolveSemicolonSign bool) any {
	pathArr := strings.Split(path, "/")
	if len(pathArr) == 0 || (len(pathArr) == 1 && pathArr[0] == "") {
		return object
	}
	m := map[string]any{}
	m1 := m

	for i := 0; i < len(pathArr); i++ {
		p := pathArr[i]
		objectType := ""
		if resolveSemicolonSign && strings.ContainsRune(p, ':') {
			pSplit := strings.Split(p, ":")
			p = pSplit[0]
			if len(pSplit) > 1 {
				objectType = pSplit[1]
			}
		}
		if i == len(pathArr)-1 {
			if objectMap, objectIsMap := object.(map[string]any); objectIsMap && objectType != "" {
				objectMap["object"] = objectType
				m1[p] = objectMap
			} else {
				m1[p] = object
			}
		} else {
			if objectType == "" {
				m1[p] = map[string]any{}
			} else {
				m1[p] = map[string]any{"object": objectType}
			}
			m1 = m1[p].(map[string]any)
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

func GetValueFromStruct(object any, path []string) (any, error) {
	v := reflect.ValueOf(object)
	for _, key := range path {
		for v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return nil, fmt.Errorf("only accepts structs; got %T", v)
		}
		v = v.FieldByName(key)
	}
	return v, nil
}

func SubstituteValuesInArray(m []any, f func(string, []string) any, path []string) {
	for i := 0; i < len(m); i++ {
		if vm, ok := m[i].(map[string]any); ok {
			SubstituteValuesInMap(vm, f, append(path, strconv.Itoa(i)))
		} else if vArr, ok := m[i].([]any); ok {
			SubstituteValuesInArray(vArr, f, append(path, strconv.Itoa(i)))
		} else if vStr, ok := m[i].(string); ok {
			m[i] = f(vStr, append(path, strconv.Itoa(i)))
		}
	}
}

func SubstituteValuesInMap(m map[string]any, f func(string, []string) any, path []string) {
	for k, v := range m {
		delete(m, k)
		k1 := fmt.Sprintf("%v", f(k, append(path, k)))
		if vm, ok := v.(map[string]any); ok {
			SubstituteValuesInMap(vm, f, append(path, k))
			m[k1] = vm
		} else if vArr, ok := v.([]any); ok {
			SubstituteValuesInArray(vArr, f, append(path, k))
			m[k1] = vArr
		} else if vStr, ok := v.(string); ok {
			m[k1] = f(vStr, append(path, k))
		} else {
			m[k1] = v
		}
	}
}

func Capitalize(str string) string {
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func EnsureMapAny(m any) map[string]any {
	result := map[string]any{}
	if mMap, mIsMap := m.(map[string]any); mIsMap {
		for k, v := range mMap {
			result[k] = v
		}
	} else if mMap, mIsMap := m.(map[string]string); mIsMap {
		for k, v := range mMap {
			result[k] = v
		}
	} else if mMap, mIsMap := m.(map[string]bool); mIsMap {
		for k, v := range mMap {
			result[k] = v
		}
	} else if mMap, mIsMap := m.(map[string]int); mIsMap {
		for k, v := range mMap {
			result[k] = v
		}
	} else if mMap, mIsMap := m.(map[string]float32); mIsMap {
		for k, v := range mMap {
			result[k] = v
		}
	} else if mMap, mIsMap := m.(map[string]float64); mIsMap {
		for k, v := range mMap {
			result[k] = v
		}
	} else {
		return nil
	}
	return result
}

func EnsureSliceAny(m any) []any {
	var result []any
	if mSl, mIsSl := m.([]any); mIsSl {
		result = make([]any, len(mSl))
		copy(result, mSl)
	} else if mSl, mIsSl := m.([]string); mIsSl {
		result = make([]any, len(mSl))
		for k, v := range mSl {
			result[k] = v
		}
	} else if mSl, mIsSl := m.([]bool); mIsSl {
		result = make([]any, len(mSl))
		for k, v := range mSl {
			result[k] = v
		}
	} else if mSl, mIsSl := m.([]int); mIsSl {
		result = make([]any, len(mSl))
		for k, v := range mSl {
			result[k] = v
		}
	} else if mSl, mIsSl := m.([]float32); mIsSl {
		result = make([]any, len(mSl))
		for k, v := range mSl {
			result[k] = v
		}
	} else if mSl, mIsSl := m.([]float64); mIsSl {
		result = make([]any, len(mSl))
		for k, v := range mSl {
			result[k] = v
		}
	} else {
		return nil
	}
	return result
}

func TryStrToNativeType(s string) any {
	i, err := strconv.Atoi(s)
	if err == nil {
		return i
	}

	f, err := strconv.ParseFloat(s, 32)
	if err == nil {
		return f
	}

	if s == "true" {
		return true
	}

	if s == "false" {
		return false
	}

	return s
}
