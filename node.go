package forjitree

import (
	"errors"
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
	Get(key string, postprocess bool) []Node
	Set(newValue any)
	Query(q any) (any, error)
	Value() any
	Parent() Node
	Root() Node
	Name() string
	Path() string
	Tree() *Tree
	NodeType() int
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

func (n *node) setNodeType(newNodeType int) bool {
	if newNodeType == n.nodeType {
		return false
	}

	n.destroyObject(true)

	n.mu.Lock()

	n.m = make(map[string]*node)
	n.sl = make([]*node, 0)
	n.value = nil

	n.nodeType = newNodeType

	n.mu.Unlock()

	return true
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
	if q == nil {
		return n.getValue(), nil
	}

	if n.nodeType == NodeTypeSlice {
		qMap := EnsureMapAny(q)
		if qMap == nil {
			return nil, errors.New("subquery must be a map or null")
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
			return nil, errors.New("subquery must be a map or null")
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
		// TODO: filtering functions similar to MySQLDatasource
		return n.getValue(), nil
	}
}

func (n *node) patch(data any) []*node {
	modified := false
	modifiedSubnodes := []*node{}

	switch d := data.(type) {
	case map[string]any:
		modified = n.setNodeType(NodeTypeMap)
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
	case []any:
		modified = n.setNodeType(NodeTypeSlice)
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
	default:
		modified = n.setNodeType(NodeTypeValue)
		n.mu.Lock()
		if n.value != data {
			modified = true
		}
		n.value = data
		n.mu.Unlock()
	}

	if modified {
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

func internalGet(nodes []*node, t pathToken, postprocess bool) []*node {
	// TODO: detect loops

	var result []*node

	appendPostprocess := func(n *node) {
		if n == nil {
			return
		}
		appendArr := []*node{n}
		if vStr, vIsStr := n.value.(string); postprocess && n.nodeType == NodeTypeValue && vIsStr && strings.HasPrefix(vStr, "@") && n.parent != nil {
			subResult := n.parent.Get(vStr[1:], true)
			appendArr = []*node{}
			for _, subNode := range subResult {
				appendArr = append(appendArr, subNode.(*node))
			}
		}
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
			for _, p := range t.Params {
				if p.Key == "PARENT_KEY" {
					if n.parentKey != p.Value {
						satisfied = false
						break
					}
				} else {
					n1 := n.getChild(p.Key)
					if n1 == nil || (p.Value != "" && n1.value != p.Value) {
						satisfied = false
						break
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

func (n *node) Get(path string, postprocess bool) []Node {
	tokenizedPath := TokenizePath(path)

	if len(tokenizedPath) == 0 {
		return []Node{n}
	}

	tempResult := []*node{n}
	for i := range tokenizedPath {
		tempResult = internalGet(tempResult, tokenizedPath[i], postprocess)
	}

	result := make([]Node, len(tempResult))
	for i := range tempResult {
		result[i] = tempResult[i]
	}
	return result
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
		return n.tree.externalPath
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
