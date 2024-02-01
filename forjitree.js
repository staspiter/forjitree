class ForjiTree {

    constructor() {
        this.objectTypes = {}
        this.rootNode = new ForjiNode(this, null, "")
        this.created = false
        this.modified = false
        this.name = ''
        this.unregisteredObjectsList = []

        this.AddTypes(ClientDatasource, Type)
        this.AddType(Block, "Block", null, /* allowUninitializedFields */ true)
    }

    Created() {
        if (this.created) 
            return
        this.rootNode.callCreatedTree()
        this.created = true
    }

    Clear() {
        this.rootNode.destroyObject(true)
        this.rootNode = new ForjiNode(this, null, "")
        this.created = false
        this.modified = true
    }

    GetValue() {
        return this.rootNode.getValue()
    }

    Set(data) {
        let modifiedNodes = this.rootNode.patch(data)

        // Call synchronize for modified nodes
        let createdObjects = []
        for (let i = modifiedNodes.length - 1; i >= 0; i--)
            if (modifiedNodes[i].synchronize()) 
                createdObjects.push(modifiedNodes[i])

        // Call CreatedChildren
        for (let i = createdObjects.length - 1; i >= 0; i--)
            createdObjects[i].callObjFunc('CreatedChildren')

        // Call CreatedTree if the tree has already been created
        if (this.created) 
            for (const o of createdObjects)
                o.callObjFunc('CreatedTree')

        if (modifiedNodes.length > 0)
            this.modified = true
    }

    AddType(objectClass, customClassName = null, defaultData = null, allowUninitializedFields = false) {
        let className = customClassName
        let classNameReal = getClassName(objectClass)

        if (className == null)
            className = classNameReal

        // Combine default data from the base class and the new class
        let defaultDataCombined = {}
        if (classNameReal != className) {
            let realObjectType = this.objectTypes[classNameReal]
            if (realObjectType) {
                // Copy default data from the base class
                for (const [k, v] of Object.entries(realObjectType.defaultData))
                    defaultDataCombined[k] = v
                
                // Use allowUninitializedFields from the base class
                if (realObjectType.allowUninitializedFields)
                    allowUninitializedFields = true
            }
        }
        if (defaultData)
            for (const [k, v] of Object.entries(defaultData))
                defaultDataCombined[k] = v

        this.objectTypes[className] = new ObjectType(objectClass, className, defaultDataCombined, allowUninitializedFields)

        // Synchronize all unregistered objects, the new type might be used in them
        let unregisteredObjectsCopy = this.unregisteredObjectsList.slice()
        this.unregisteredObjectsList = []
        for (const p of unregisteredObjectsCopy) {
            let nodes = this.rootNode.Get(p)
            for (const n of nodes)
                n.synchronize()
        }
    }

    AddTypes(...objectClasses) {
        for (const t of objectClasses)
            this.AddType(t)
    }

    GetType(name) {
        let t = this.objectTypes[name]
        if (t)
            return t
        return null
    }

    Root() {
        return this.rootNode
    }

    IsModified() {
        return this.modified
    }

    ResetModified() {
        this.modified = false
    }

    SetName(newName) {
        this.name = newName
    }

    GetName() {
        return this.name
    }

}

const NodeType = {
    Map: 0,
    Slice: 1,
    Value: 2,
}

const ObjectKeyword = "object"

class ForjiNode {

    constructor(tree, parent, parentKey) {
        this.tree = tree
        this.parent = parent
        this.parentKey = parentKey
        this.nodeType = NodeType.Value
        this.m = {}
        this.sl = []
        this.value = null
        this.obj = null
        this.objType = null
    }

    setNodeType(newNodeType) {
        if (newNodeType == this.nodeType)
            return false

        this.destroyObject(true)

        this.m = {}
        this.sl = []
        this.value = null

        this.nodeType = newNodeType

        return true
    }

    getValue() {
        if (this.nodeType == NodeType.Map) {
            let m = {}
            for (const [k, v] of Object.entries(this.m))
                m[k] = v.getValue()
            return m
        } else if (this.nodeType == NodeType.Slice) {
            let sl = []
            for (const v of this.sl)
                sl.push(v.getValue())
            return sl
        } else {
            return this.value
        } 
    }

    patch(d) {
        let modified = false
        let modifiedSubnodes = []

        if (d !== null && typeof d === 'object' && !Array.isArray(d)) {
            modified = this.setNodeType(NodeType.Map)
            for (const [k, v] of Object.entries(d)) {
                let n = this.m[k]
                if (!n) {
                    n = new ForjiNode(this.tree, this, k)
                    this.m[k] = n
                    modified = true
                }
                modifiedSubnodes = modifiedSubnodes.concat(n.patch(v))
            }

        } else if (d !== null && Array.isArray(d)) {
            modified = this.setNodeType(NodeType.Slice)

            // Check if we are in appendArray mode
            let appendArrayMode = d.length > 0 && typeof d[0] === 'object' && d[0]["appendArray"]

            if (appendArrayMode) {
                for (let i = 0; i < d.length; i++) {
                    let v = d[i]
                    let n = new ForjiNode(this.tree, this, this.sl.length)
                    this.sl.push(n)
                    modified = true
                    modifiedSubnodes = modifiedSubnodes.concat(n.patch(v))
                }
            } else {
                for (let i = 0; i < d.length; i++) {
                    let v = d[i]
                    if (this.sl.length <= i) {
                        let n = new ForjiNode(this.tree, this, i)
                        this.sl.push(n)
                        modified = true
                    }
                    let n = this.sl[i]
                    modifiedSubnodes = modifiedSubnodes.concat(n.patch(v))
                }
                if (this.sl.length > d.length) {
                    for (let i = d.length; i < this.sl.length; i++)
                        this.sl[i].destroyObject(true)
                    this.sl = this.sl.slice(0, d.length)
                    modified = true
                }                
            }

        } else {
            modified = this.setNodeType(NodeType.Value)
            if (this.value != d || (typeof this.value !== typeof d))
                modified = true
            this.value = d
        }

        if (modified || modifiedSubnodes.length > 0)
            modifiedSubnodes.push(this)

        return modifiedSubnodes
    }

    synchronize() {
        let createdObj = false

        let newType = null
        if (this.nodeType == NodeType.Map) {
            let typeNode = this.m[ObjectKeyword]
            if (typeNode && typeNode.nodeType == NodeType.Value && typeNode.value != null) {
                newType = this.tree.GetType(typeNode.value)
                if (!newType) {
                    let p = this.Path()
                    if (!this.tree.unregisteredObjectsList.includes(p))
                        this.tree.unregisteredObjectsList.push(p)                    
                }
            }
        }

        if (newType != this.objType || 
            (this.objType && typeof this.objType.Immutable === 'function' && this.objType.Immutable()) || 
            (newType && typeof newType.Immutable === 'function' && newType.Immutable())) {

            if (this.objType)
                this.destroyObject(false)
            
            if (newType) {
                this.objType = newType
                this.obj = newType.createObject(this)
                this.obj.node = this

                // Set default fields before calling Created
                if (this.objType.defaultData)
                    for (const [k, v] of Object.entries(this.objType.defaultData))
                        this.objType.setField(this, k, v)

                // Set all fields before calling Created
                for (const [k, v] of Object.entries(this.m)) {
                    if (k == ObjectKeyword)
                        continue
                    this.objType.setField(this, k, v.getValue())
                }

                this.callObjFunc('Created')
                createdObj = true
            }
        }

        if (this.parent != null && this.parent.nodeType == NodeType.Map && this.parent.objType != null && this.parentKey != ObjectKeyword)
            this.parent.objType.setField(this.parent, this.parentKey, this.getValue())
        
        return createdObj
    }

    destroyObject(callNested) {
        if (callNested) {
            for (const [k, v] of Object.entries(this.m))
                v.destroyObject(true)
            for (const v of this.sl)
                v.destroyObject(true)
        }

        if (this.objType) {
            this.callObjFunc('Destroyed')
            this.objType = null
            this.obj = null
        }
    }

    callCreatedTree() {
        if (this.objType) 
            this.callObjFunc('CreatedTree')
        for (const [k, v] of Object.entries(this.m))
            v.callCreatedTree()
        for (const v of this.sl)
            v.callCreatedTree()
    }

    callObjFunc(funcName) {
        if (!this.obj || !this.obj[funcName] || typeof this.obj[funcName] !== "function")
            return
        this.obj[funcName]()
    }

    getParent() {
        return this.parent
    }

    getChild(key) {
        if (this.nodeType == NodeType.Map)
            return this.m[key]
        else if (this.nodeType == NodeType.Slice)
            return this.sl[key]
        else
            return null
    }

    getChildren(recursive) {
        let result = []
        if (this.nodeType == NodeType.Map) {
            for (const [k, v] of Object.entries(this.m)) {
                result.push(v)
                if (recursive)
                    result = result.concat(v.getChildren(true))
            }
        } else if (this.nodeType == NodeType.Slice) {
            for (const v of this.sl) {
                result.push(v)
                if (recursive)
                    result = result.concat(v.getChildren(true))
            }
        }
        return result
    }

    getParents() {
        let result = []
        let p = this.parent
        while (p) {
            result.push(p)
            p = p.parent
        }
        return result
    }

    internalGet(nodes, t, postProcess) {
        let result = []

        let appendPostprocess = (n) => {
            if (n == null)
                return
            let appendArr = [n]
            if (n.value && (typeof n.value === 'string' || n.value instanceof String) && n.value.startsWith('@') && n.parent != null) {
                let subResult = n.parent.Get(n.value.substring(1), true)
                appendArr = []
                for (const sn of subResult)
                    appendArr.push(sn)
            }
            for (const an of appendArr) {
                let exists = false
                for (const r of result)
                    if (r == an) {
                        exists = true
                        break
                    }
                if (!exists)
                    result.push(an)
            }
        }

        for (const n of nodes) {

            if (t.kind == PathTokenKind.This)
                appendPostprocess(n)
            
            else if (t.kind == PathTokenKind.Parent)
                appendPostprocess(n.parent)

            else if (t.kind == PathTokenKind.AllParents) {
                let parents = n.getParents()
                for (const p of parents)
                    appendPostprocess(p)
            }

            else if (t.kind == PathTokenKind.Root)
                appendPostprocess(n.tree.rootNode)

            else if (t.kind == PathTokenKind.Sub)
                appendPostprocess(n.getChild(t.key))

            else if (t.kind == PathTokenKind.Params) {
                let satisfied = true
                for (const p of t.params) {
                    if (p.key == "PARENT_KEY") {
                        if (n.parentKey != p.value) {
                            satisfied = false
                            break
                        }
                    } else {
                        let n1 = n.getChild(p.key)
                        if (!n1 || (p.value != "" && n1.value != p.value)) {
                            satisfied = false
                            break
                        }
                    }
                }
                if (satisfied)
                    appendPostprocess(n)
            }

            else if (t.kind == PathTokenKind.DirectChildren) {
                let subs = n.getChildren(false)
                for (const s of subs)
                    appendPostprocess(s)
            }

            else if (t.kind == PathTokenKind.AllChildren) {
                let subs = n.getChildren(true)
                for (const s of subs)
                    appendPostprocess(s)
            }
        }

        return result
    }

    Get(path, postProcess) {
        let tokenizedPath = tokenizePath(path)

        if (tokenizedPath.length == 0)
            return []

        let tempResult = [this]
        for (const t of tokenizedPath)
            tempResult = this.internalGet(tempResult, t, postProcess)

        return tempResult
    }

    GetOne(path, postProcess) {
        let arr = this.Get(path, postProcess)
        if (arr.length == 0)
            return null
        return arr[0]
    }

    Value() {
        return this.getValue()
    }

    Parent() {
        return this.getParent()
    }

    Root() {
        return this.tree.rootNode
    }

    Name() {
        return this.parentKey
    }

    Path() {
        if (this.parent == null)
            return ""
        return this.parent.Path() + "/" + this.parentKey
    }

    Tree() {
        return this.tree
    }

    Set(newValue) {
        this.tree.Set(makePatchWithPath(this.Path().substring(1), newValue))
    }

    NodeType() {
        return this.nodeType
    }
}

class ObjectType {

    constructor(objectClass, name, defaultData, allowUninitializedFields = false) {
        this.objectClass = objectClass
        this.name = name
        this.defaultData = defaultData
        this.allowUninitializedFields = allowUninitializedFields
    }

    createObject(node) {
        return new this.objectClass(node)
    }

    setField(node, key, value) {
        // By default we require fields in the object to be initialized
        // This is allowed to all blocks
        if (!this.allowUninitializedFields && typeof node.obj[key] === 'undefined')
            return

        node.obj[key] = value
        
        if (node.obj["Updated"] && typeof node.obj["Updated"] === "function")
            node.obj["Updated"](key, value)
    }

}

const PathTokenKind = {
    This: 0,
    Parent: 1,
    Root: 2,
    Sub: 3,
    Params: 4,
    DirectChildren: 5,
    AllChildren: 6,
    AllParents: 7,
}

function tokenizePath(path) {
    let tokensDelimeters = "/["
    let tokensStr = []
    let t = ""
    for (let i = 0; i < path.length; i++) {
        if (tokensDelimeters.includes(path[i])) {
            tokensStr.push(t)
            t = ""
        }
        t += path[i]
    }
    tokensStr.push(t)

    let tokens = []
    for (let i = 0; i < tokensStr.length; i++) {
        let ts = tokensStr[i]
        let t = {
            kind: PathTokenKind.This,
            params: []
        }

        if (ts == "" && i == 0 && tokensStr.length > 1 && tokensStr[1].startsWith("/"))
            t.kind = PathTokenKind.Root

        else if ((ts == "@" && i == 0) || ts == "/@")
            t.kind = PathTokenKind.This

        else if ((ts == "!.." && i == 0) || ts == "/!..")
            t.kind = PathTokenKind.Root

        else if ((ts == ".." && i == 0) || ts == "/..")
            t.kind = PathTokenKind.Parent

        else if ((ts == "..." && i == 0) || ts == "/...")
            t.kind = PathTokenKind.AllParents

        else if ((ts == "*" && i == 0) || ts == "/*")
            t.kind = PathTokenKind.DirectChildren

        else if ((ts == "**" && i == 0) || ts == "/**")
            t.kind = PathTokenKind.AllChildren

        else if (ts.startsWith("[") && ts.endsWith("]")) {
            // [key=value,key] filter token
            t.kind = PathTokenKind.Params
            ts = ts.substring(1, ts.length - 1)
            let pairs = Array.from(ts.matchAll(/[^",]+|"([^"]*)"/g), ([a,b]) => b || a);
            for (const p of pairs) {
                if (p.includes("=")) {
                    // Check for key and value
                    let equationPos = p.indexOf("=")
                    let key = p.substring(0, equationPos)
                    let value = p.substring(equationPos + 1)
                    t.params.push({key: key, value: value})
                } else {
                    // Check for key presense
                    t.params.push({key: p, value: ""})
                }
            }
        }

        else if (ts.startsWith("/")) {
            if (ts.length > 1) {
                t.kind = PathTokenKind.Sub
                t.key = ts.substring(1)
            } else
                t.kind = PathTokenKind.This
        }

        else if (i == 0 && ts != "") {
            t.kind = PathTokenKind.Sub
            t.key = ts
        }

        tokens.push(t)
    }

    return tokens
}

function makePatchWithPath(path, object) {
    if (path == "")
        return object
    let pathArr = path.split('/')
    let m = {}
    let m1 = m
    for (let i = 0; i < pathArr.length; i++) {
        if (i == pathArr.length - 1)
            m1[pathArr[i]] = object
        else {
            m1[pathArr[i]] = {}
            m1 = m1[pathArr[i]]
        }
    }
    return m
}

function GetObjs(nodes, typeCheck = null) {
    let result = []
    if (Array.isArray(nodes)) {
        for (const n of nodes)
            if (n.obj && (typeCheck == null || n.obj instanceof typeCheck))
                result.push(n.obj)
    } else {
        if (nodes.obj && (typeCheck == null || nodes.obj instanceof typeCheck))
            result.push(nodes.obj)
    }
    return result
}

function GetObj(nodes, typeCheck = null) {
    if (Array.isArray(nodes)) {
        for (const n of nodes)
            if (n.obj && (typeCheck == null || n.obj instanceof typeCheck))
                return n.obj        
    } else {
        if (nodes.obj && (typeCheck == null || nodes.obj instanceof typeCheck))
            return nodes.obj
    }

    return null
}

function getClassName(c) {
    if (c.TypeName)
        return c.TypeName()
    let match = /class\s+(?<classname>.+)\s+{/mg.exec(c.toString())
    if (match !== null)
        return match.groups.classname.replace(/ .*/,'') // Get the first word to exclude "extends..."
    return null
}

class ClientDatasource {

    constructor(node) {
        this.node = node
        this.url = ""
        this.Tree = new ForjiTree()
        this.watcherId = crypto.randomUUID()
        this.reconnectTimer = null
        this.OnError = null
        this.OnClose = null
    }

    Created() {
        this.Connect()
    }

    Destroyed() {
        this.Disconnect()
        this.Tree.Clear()
    }

    Connect() {
        let self = this

        this.Tree.SetName(this.url)

        if (this.url.startsWith('http://') || this.url.startsWith('https://')) {
            // Request data only once
            fetch(this.url)
                .then(response => response.json())
                .then(data => self.Tree.Set(data))
        }

        else if (this.url.startsWith('ws://') || this.url.startsWith('wss://')) {
            // Establish a ws connection
            let url = this.url
            if (this.url.includes('?'))
                url += '&'
            else
                url += '?'
            url += 'watcherId=' + this.watcherId
            
            this.socket = new WebSocket(url)
            this.socket.binaryType = 'arraybuffer'
            this.socket.onopen = (event) => {
                if (self.reconnectTimer != null) {
                    clearInterval(self.reconnectTimer)
                    self.reconnectTimer = null
                }
            }
            this.socket.onmessage = (event) => {
                let data = msgpack.deserialize(new Uint8Array(event.data))
                self.Tree.Set(data)
            }
            this.socket.onclose = (event) => {
                if (self.reconnectTimer == null)
                    self.reconnectTimer = setInterval(() => { self.Connect() }, 2000)
                if (self.OnClose)
                    self.OnClose(event)
            }
            this.socket.onerror = (error) => {
                if (self.reconnectTimer == null)
                    self.reconnectTimer = setInterval(() => { self.Connect() }, 2000)
                if (self.OnError)
                    self.OnError(error)
            }
        }
    }

    Disconnect() {
        if (this.socket) {
            if (this.reconnectTimer)
                clearInterval(this.reconnectTimer)
            this.socket.close()
        }        
    }

}

class Type {

    constructor(node) {
        this.node = node
        this.base = ""
        this.name = ""
        this.defaultData = {}
    }

    Created() {
        this.name = this.node.Name()

        console.log("Registering dynamic type " + this.name)

        // Retrieve default type data from the nodes
        this.defaultData = {}
        for (const [k, v] of Object.entries(this.node.m))
            if (k != ObjectKeyword && k != "base")
                this.defaultData[k] = v.getValue()

        // Get the base type from the tree
        let baseType = this.node.Tree().objectTypes[this.base]
        if (!baseType) {
            console.warn("Base type " + this.base + " was not found")
            return
        }

        // Add the type to the tree
        this.node.tree.AddType(baseType.objectClass, this.name, this.defaultData)
    }

    Destroyed() {
        this.node.tree.objectTypes[this.name] = null
    }

}

class Block {

    constructor(node) {
        this.node = node
        this.html = ""
        this.container = ""
        
        this.element = document.createElement('div')
        this.element.classList.add('block')
        this.element.setAttribute('type', this.node.objType.name)
        this.created = false
    }

    Created() {
        // Find parent Block
        this.parentBlock = GetObj(this.node.Get("...[object=Block]"))

        let appended = false

        // Append to parent Block if it exists
        if (this.parentBlock) {
            let containerElement = this.parentBlock.element.querySelector("[contains*='" + this.node.objType.name + "']")
            if (containerElement != null) {
                this.element = containerElement.appendChild(this.element)
                appended = true
            }
            else
                console.warn("Could not append block " + this.node.Name() + " (" + this.node.objType.name + 
                    "), container not found in parent block " + this.parentBlock.node.Name() + " (" + this.parentBlock.node.objType.name + ")")
        }

        // Append to container if it exists
        if (!appended && this.container) {
            let containerElement = document.querySelector(this.container)
            if (containerElement != null) {
                this.element = containerElement.appendChild(this.element)
                appended = true
            }
            else
                console.warn("Could not append block " + this.node.Name() + " (" + this.node.objType.name + "), container not found: " + this.container)
        }

        // Append to body
        if (!appended)
            this.element = document.body.appendChild(this.element)

        this.created = true

        this.Updated()
    }

    Destroyed() {
        this.element.remove()
    }

    Updated() { 
        if (!this.created) 
            return

        // Remember all contained block elements to put them back after refilling the block
        let containedBlocks = []
        for (const e of this.element.querySelectorAll('.block'))
            containedBlocks.push(e)

        this.element.innerHTML = interpolateTemplate(this.html, this.node.getValue())

        // Put the contained blocks back
        let containerElement = null
        for (const e of containedBlocks)
            if ((containerElement = this.element.querySelector("[contains*='" + e.getAttribute("type") + "']")) != null)
                containerElement.appendChild(e)
    }

}

function interpolateTemplate(tpl, args) {
    var keys = Object.keys(args), fn = new Function(...keys, 'return `' + tpl.replace(/`/g, '\\`') + '`')
    return fn(...keys.map(x => args[x]))
}