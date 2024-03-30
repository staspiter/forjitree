package forjitree

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

const (
	NodeTypeMap = iota
	NodeTypeSlice
	NodeTypeValue
)

const ObjectKeyword = "object"

type node struct {
	tree      *Tree
	parent    *node
	parentKey string

	value    any
	m        map[string]*node
	sl       []*node
	nodeType int
	mu       sync.RWMutex

	obj        Object
	objReflect reflect.Value
	objType    *ObjectType
}

type Node interface {
	Get(key string) []Node
	GetEx(key string, links bool, redirects bool, avoidDuplicates bool) []Node
	GetOne(key string) Node
	Set(newValue any)
	Query(q any) (any, error)
	Value() any
	Parent() Node
	Root() Node
	Name() string
	Path() string
	Tree() *Tree
	NodeType() int
	CleanNulls(recursive bool)
	internalNode() *node
}

func newNode(tree *Tree, parent *node, parentKey string) *node {
	n := &node{
		tree:      tree,
		parent:    parent,
		parentKey: parentKey,
		nodeType:  NodeTypeValue,
	}
	return n
}

func (n *node) setNodeType(newNodeType int) (modified bool, allowed bool) {
	if newNodeType == n.nodeType {
		return false, true
	}

	n.destroyObject(true)

	n.mu.Lock()

	n.m = make(map[string]*node)
	n.sl = make([]*node, 0)
	n.value = nil

	n.nodeType = newNodeType

	n.mu.Unlock()

	return true, true
}

func (n *node) getValue() any {
	switch n.nodeType {
	case NodeTypeMap:
		m := make(map[string]any)
		n.mu.RLock()
		for k, v := range n.m {
			n.mu.RUnlock()
			m[k] = v.getValue()
			n.mu.RLock()
		}
		n.mu.RUnlock()
		return m
	case NodeTypeSlice:
		sl := make([]any, len(n.sl))
		n.mu.RLock()
		for i, v := range n.sl {
			n.mu.RUnlock()
			sl[i] = v.getValue()
			n.mu.RLock()
		}
		n.mu.RUnlock()
		return sl
	case NodeTypeValue:
		return n.value
	}
	panic("invalid node type")
}

func (n *node) query(q any) (any, error) {
	// Return the whole subtree (value)
	if q == nil {
		return n.getValue(), nil
	}

	// String query
	if qStr, qIsStr := q.(string); qIsStr {
		nodes := n.GetEx(qStr, true, true, true)
		result := map[string]any{}
		for _, n := range nodes {

			// Get the full path of each node including the name of the tree (if defined)
			treeName := n.Tree().GetName()
			path := n.Path()
			fullPath := treeName
			if len(treeName) > 0 && len(path) > 0 {
				fullPath += "/"
			}
			fullPath += path

			patch := MakePatchWithPath(fullPath, n.Value(), true)
			if patchMap, ok := patch.(map[string]any); ok {
				MergeMaps(result, patchMap)
			}
		}
		return result, nil
	}

	// Object path query

	if n.nodeType == NodeTypeSlice {
		qMap := EnsureMapAny(q)
		if qMap == nil {
			return nil, errors.New("map expected in the subquery")
		}

		children := n.getChildren(false)
		result := []any{}
		for _, child := range children {
			item, err := child.query(q)
			if err == nil {
				result = append(result, item)
			}
		}

		return result, nil

	} else if n.nodeType == NodeTypeMap {
		qMap := EnsureMapAny(q)
		if qMap == nil {
			return nil, errors.New("map expected in the subquery")
		}

		result := map[string]any{}
		for k, v := range qMap {
			child := n.getChild(k)
			if child == nil {
				result[k] = nil
				continue
			}
			item, err := child.query(v)
			if err == nil {
				result[k] = item
			}
		}

		return result, nil

	} else {
		return n.getValue(), nil
	}
}

func (n *node) patch(data any) []*node {
	modified := false
	allowed := false
	modifiedSubnodes := []*node{}

	switch d := data.(type) {
	case map[string]any:
		modified, allowed = n.setNodeType(NodeTypeMap)
		if allowed {
			for k, v := range d {
				n.mu.Lock()
				if _, ok := n.m[k]; !ok {
					n.m[k] = newNode(n.tree, n, k)
					modified = true
				}
				subnode := n.m[k]
				n.mu.Unlock()
				modifiedSubnodes = append(modifiedSubnodes, subnode.patch(v)...)
			}
		}

	case []any:
		modified, allowed = n.setNodeType(NodeTypeSlice)

		if allowed {

			// Check if we are in appendArray mode
			appendArrayMode := false
			if len(d) > 0 {
				firstItem := d[0]
				if firstItemMap, firstItemIsMap := firstItem.(map[string]any); firstItemIsMap {
					if _, firstItemIsMapWithAppendArray := firstItemMap["appendArray"]; firstItemIsMapWithAppendArray {
						appendArrayMode = true
					}
				}
			}

			if appendArrayMode {
				for _, v := range d {
					n.mu.Lock()
					subnode := newNode(n.tree, n, strconv.Itoa(len(n.sl)))
					n.sl = append(n.sl, subnode)
					n.mu.Unlock()
					modified = true
					modifiedSubnodes = append(modifiedSubnodes, subnode.patch(v)...)
				}
			} else {
				for i, v := range d {
					n.mu.Lock()
					if len(n.sl) <= i {
						n.sl = append(n.sl, newNode(n.tree, n, strconv.Itoa(i)))
						modified = true
					}
					subnode := n.sl[i]
					n.mu.Unlock()
					modifiedSubnodes = append(modifiedSubnodes, subnode.patch(v)...)
				}
				if len(n.sl) > len(d) {
					n.mu.Lock()
					slCopy := make([]*node, len(n.sl))
					copy(slCopy, n.sl)
					n.sl = n.sl[:len(d)]
					n.mu.Unlock()
					for i := len(d); i < len(slCopy); i++ {
						slCopy[i].destroyObject(true)
					}
					modified = true
				}
			}
		}

	default:
		modified, _ = n.setNodeType(NodeTypeValue)
		n.mu.Lock()
		if n.value != data {
			modified = true
		}
		n.value = data
		n.mu.Unlock()
	}

	if modified || len(modifiedSubnodes) > 0 {
		return append(modifiedSubnodes, n)
	} else {
		return modifiedSubnodes
	}
}

func (n *node) synchronize() bool {
	var createdObj = false

	var newType *ObjectType = nil
	if n.nodeType == NodeTypeMap {
		n.mu.RLock()
		typeNode, typeNodeExists := n.m[ObjectKeyword]
		if typeNodeExists && typeNode.nodeType == NodeTypeValue && typeNode.value != nil {
			if typeValue, typeValueIsStr := typeNode.value.(string); typeValueIsStr {
				newType = n.tree.GetType(typeValue)
			}
		}
		n.mu.RUnlock()
	}

	if newType != n.objType {
		if n.objType != nil {
			n.destroyObject(false)
		}

		if newType != nil {
			n.objType = newType
			n.obj = newType.createObject(n)
			n.objReflect = reflect.ValueOf(n.obj)

			// Set all fields immediately before calling Created
			n.mu.RLock()
			for k, v := range n.m {
				if k == ObjectKeyword {
					continue
				}
				n.mu.RUnlock()
				n.objType.setField(n, k, v.getValue())
				n.mu.RLock()
			}
			n.mu.RUnlock()

			n.obj.Created()
			createdObj = true
		}
	}

	if n.parent != nil && n.parent.nodeType == NodeTypeMap && n.parent.objType != nil && n.parentKey != ObjectKeyword {
		n.parent.objType.setField(n.parent, n.parentKey, n.getValue())
	}

	return createdObj
}

func (n *node) destroyObject(callNested bool) {
	if callNested {
		switch n.nodeType {
		case NodeTypeMap:
			n.mu.RLock()
			for _, v := range n.m {
				n.mu.RUnlock()
				v.destroyObject(true)
				n.mu.RLock()
			}
			n.mu.RUnlock()
		case NodeTypeSlice:
			n.mu.RLock()
			for _, v := range n.sl {
				n.mu.RUnlock()
				v.destroyObject(true)
				n.mu.RLock()
			}
			n.mu.RUnlock()
		}
	}

	if n.objType != nil {
		n.obj.Destroyed()
		n.objType = nil
		n.obj = nil
		n.objReflect = reflect.Value{}
	}
}

func (n *node) callCreatedTree() {
	if n.objType != nil {
		n.obj.CreatedTree()
	}

	switch n.nodeType {
	case NodeTypeMap:
		n.mu.RLock()
		for _, v := range n.m {
			n.mu.RUnlock()
			v.callCreatedTree()
			n.mu.RLock()
		}
		n.mu.RUnlock()
	case NodeTypeSlice:
		n.mu.RLock()
		for _, v := range n.sl {
			n.mu.RUnlock()
			v.callCreatedTree()
			n.mu.RLock()
		}
		n.mu.RUnlock()
	}
}

func (n *node) getParent() *node {
	return n.parent
}

func (n *node) getChild(key string) *node {
	var result *node = nil

	switch n.nodeType {
	case NodeTypeMap:
		n.mu.RLock()
		if v, ok := n.m[key]; ok {
			result = v
		}
		n.mu.RUnlock()
	case NodeTypeSlice:
		n.mu.RLock()
		if i, err := strconv.Atoi(key); err == nil && i >= 0 && i < len(n.sl) {
			result = n.sl[i]
		}
		n.mu.RUnlock()
	}

	return result
}

func (n *node) getChildren(recursive bool) []*node {
	var result []*node

	switch n.nodeType {
	case NodeTypeMap:
		n.mu.RLock()
		for _, v := range n.m {
			n.mu.RUnlock()
			result = append(result, v)
			if recursive {
				result = append(result, v.getChildren(true)...)
			}
			n.mu.RLock()
		}
		n.mu.RUnlock()
	case NodeTypeSlice:
		n.mu.RLock()
		for _, v := range n.sl {
			n.mu.RUnlock()
			result = append(result, v)
			if recursive {
				result = append(result, v.getChildren(true)...)
			}
			n.mu.RLock()
		}
		n.mu.RUnlock()
	}

	return result
}

func (n *node) getParents() []*node {
	var result []*node

	if n.parent != nil {
		result = append(result, n.parent)
		result = append(result, n.parent.getParents()...)
	}

	return result
}

func internalGet(nodes []*node, t pathToken, links bool, redirects bool, avoidDuplicates bool) []*node {
	// TODO: detect loops

	var result []*node

	appendPostprocess := func(n *node) {
		if n == nil {
			return
		}

		var appendArr []*node

		if vStr, vIsStr := n.value.(string); links && n.nodeType == NodeTypeValue && vIsStr && strings.HasPrefix(vStr, "@") && n.parent != nil {
			// Links (string values starting with @)
			subResult := n.parent.GetEx(vStr[1:], links, redirects, avoidDuplicates)
			for _, subNode := range subResult {
				appendArr = append(appendArr, subNode.(*node))
			}
		} else if redirects && n.objType != nil {
			// Object redirect (for subtrees support)
			objLink, objLinkSupported := n.obj.(ObjectLink)
			if objLinkSupported {
				redirectNodes := objLink.Redirect()
				for _, n2 := range redirectNodes {
					appendArr = append(appendArr, n2.internalNode())
				}
			}

		} else {
			appendArr = []*node{n}
		}

		if avoidDuplicates {
			for _, n2 := range appendArr {
				exists := false
				for i := 0; i < len(result); i++ {
					if result[i] == n2 {
						exists = true
						return
					}
				}
				if !exists {
					result = append(result, n2)
				}
			}
		} else {
			result = append(result, appendArr...)
		}
	}

	for _, n := range nodes {

		if t.Kind == PathTokenKindThis {
			appendPostprocess(n)

		} else if t.Kind == PathTokenKindParent {
			appendPostprocess(n.Parent().(*node))

		} else if t.Kind == PathTokenKindAllParents {
			parents := n.getParents()
			for _, p := range parents {
				appendPostprocess(p)
			}

		} else if t.Kind == PathTokenKindRoot {
			appendPostprocess(n.tree.rootNode)

		} else if t.Kind == PathTokenKindSub {
			appendPostprocess(n.getChild(t.Key))

		} else if t.Kind == PathTokenKindParams {
			satisfied := true

		loop:
			for _, p := range t.Params {

				switch p.ParamType {
				case ParamTypeEquals:
					if p.Key == "_key" {
						if n.parentKey != p.Value {
							satisfied = false
							break loop
						}
					} else {
						n1 := n.GetOne(p.Key)
						if fmt.Sprintf("%v", n1.Value()) != p.Value {
							satisfied = false
							break loop
						}
					}

				case ParamTypeNotEquals:
					n1 := n.GetOne(p.Key)
					if fmt.Sprintf("%v", n1.Value()) == p.Value {
						satisfied = false
						break loop
					}

				case ParamTypeGreaterThan:
					n1 := n.GetOne(p.Key)
					n1FloatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", n1.Value()), 64)
					if err != nil {
						satisfied = false
						break loop
					}
					pFloatValue, err := strconv.ParseFloat(p.Value, 64)
					if err != nil {
						satisfied = false
						break loop
					}
					if n1FloatValue <= pFloatValue {
						satisfied = false
						break loop
					}

				case ParamTypeLessThan:
					n1 := n.GetOne(p.Key)
					n1FloatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", n1.Value()), 64)
					if err != nil {
						satisfied = false
						break loop
					}
					pFloatValue, err := strconv.ParseFloat(p.Value, 64)
					if err != nil {
						satisfied = false
						break loop
					}
					if n1FloatValue >= pFloatValue {
						satisfied = false
						break loop
					}

				case ParamTypeGreaterOrEquals:
					n1 := n.GetOne(p.Key)
					n1FloatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", n1.Value()), 64)
					if err != nil {
						satisfied = false
						break loop
					}
					pFloatValue, err := strconv.ParseFloat(p.Value, 64)
					if err != nil {
						satisfied = false
						break loop
					}
					if n1FloatValue < pFloatValue {
						satisfied = false
						break loop
					}

				case ParamTypeLessOrEquals:
					n1 := n.GetOne(p.Key)
					n1FloatValue, err := strconv.ParseFloat(fmt.Sprintf("%v", n1.Value()), 64)
					if err != nil {
						satisfied = false
						break loop
					}
					pFloatValue, err := strconv.ParseFloat(p.Value, 64)
					if err != nil {
						satisfied = false
						break loop
					}
					if n1FloatValue > pFloatValue {
						satisfied = false
						break loop
					}

				case ParamTypePresence:
					n1 := n.GetOne(p.Key)
					if n1 == nil {
						satisfied = false
						break loop
					}

				case ParamTypeNotPresence:
					n1 := n.GetOne(p.Key)
					if n1 != nil {
						satisfied = false
						break loop
					}

				case ParamTypeRegex:
					n1 := n.GetOne(p.Key)
					if n1 == nil || p.ValueRegex == nil {
						satisfied = false
						break loop
					}
					n1Str := fmt.Sprintf("%v", n1.Value())
					if !p.ValueRegex.MatchString(n1Str) {
						satisfied = false
						break loop
					}
				}

			}
			if satisfied {
				appendPostprocess(n)
			}

		} else if t.Kind == PathTokenKindDirectChildren {
			subs := n.getChildren(false)
			for _, sub := range subs {
				appendPostprocess(sub)
			}

		} else if t.Kind == PathTokenKindAllChildren {
			subs := n.getChildren(true)
			for _, sub := range subs {
				appendPostprocess(sub)
			}
		}

	}

	return result
}

func (n *node) GetEx(path string, links bool, redirects bool, avoidDuplicates bool) []Node {
	tokenizedPath := TokenizePath(path)

	if len(tokenizedPath) == 0 {
		return []Node{n}
	}

	tempResult := []*node{n}
	for i := range tokenizedPath {
		tempResult = internalGet(tempResult, tokenizedPath[i], links, redirects, avoidDuplicates)
	}

	result := make([]Node, len(tempResult))
	for i := range tempResult {
		result[i] = tempResult[i]
	}
	return result
}

func (n *node) Get(path string) []Node {
	return n.GetEx(path, true, true, true)
}

func (n *node) GetOne(path string) Node {
	arr := n.Get(path)
	if len(arr) == 0 {
		return nil
	}
	return arr[0]
}

func (n *node) Query(q any) (any, error) {
	return n.query(q)
}

func (n *node) Value() any {
	return n.getValue()
}

func (n *node) Parent() Node {
	return n.getParent()
}

func (n *node) Root() Node {
	return n.tree.rootNode
}

func (n *node) Name() string {
	if n.parent == nil {
		return n.tree.name
	}
	return n.parentKey
}

func (n *node) Path() string {
	if n.parent == nil {
		return ""
	}
	return n.parent.Path() + "/" + n.parentKey
}

func (n *node) Tree() *Tree {
	return n.tree
}

func (n *node) Set(newValue any) {
	n.tree.Set(MakePatchWithPath(strings.TrimPrefix(n.Path(), "/"), newValue, false))
}

func (n *node) NodeType() int {
	return n.nodeType
}

func (n *node) CleanNulls(recursive bool) {
	subs := n.getChildren(recursive)

	for _, n2 := range subs {
		if n2.nodeType == NodeTypeValue && n2.value == nil {
			p := n2.parent
			pKey := n2.parentKey

			if p.nodeType == NodeTypeMap {
				p.mu.Lock()
				delete(p.m, pKey)
				p.mu.Unlock()
			}
		}
	}
}

func (n *node) internalNode() *node {
	return n
}

func GetObjs[T any](nodes []Node) []T {
	result := []T{}
	for _, nIntf := range nodes {
		n, isNode := nIntf.(*node)
		if !isNode || n.objType == nil {
			continue
		}
		if obj, objIsRightType := n.obj.(T); objIsRightType {
			result = append(result, obj)
		}
	}
	return result
}

func GetObj[T any](nodes []Node) T {
	for _, nIntf := range nodes {
		n, isNode := nIntf.(*node)
		if !isNode || n.objType == nil {
			continue
		}
		if obj, objIsRightType := n.obj.(T); objIsRightType {
			return obj
		}
	}
	var result T
	return result
}
