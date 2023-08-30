class Tree {

    constructor() {
        this.objectTypes = {}
        this.rootNode = new Node(this, null, "")
        this.created = false
        this.modified = false

    }

    Created() {
        if (this.created) 
            return
        this.rootNode.callCreatedTree()
        this.created = true
    }

    Clear() {
        this.rootNode.destroyObject(true)
        this.rootNode = new Node(this, null, "")
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
            createdObjects[i].callObj('CreatedChildren')

        // Call CreatedTree if the tree has already been created
        if (this.created) 
            for (const o of createdObjects)
                o.callObj('CreatedTree')

        if (modifiedNodes.length > 0)
            this.modified = true
    }

    AddType(objectClass, name) {
        this.objectTypes[name] = new ObjectType(objectClass, name)
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

}

class Node {

    constructor(tree, parent, parentKey) {
        
    }

    callCreatedTree() {

    }

    destroyObject(callNested) {

    }

    callObj(funcName) {
        
    }

    getValue() {

    }

}

class ObjectType {

    constructor(objectClass, name) {

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
    AllParents: 7
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

class DatasourceClient {

}