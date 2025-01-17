package xmlquery

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"strings"
)

// A NodeType is the type of a Node.
type NodeType uint

const (
	// DocumentNode is a document object that, as the root of the document tree,
	// provides access to the entire XML document.
	DocumentNode NodeType = iota
	// DeclarationNode is the document type declaration, indicated by the
	// following tag (for example, <!DOCTYPE...> ).
	DeclarationNode
	// ElementNode is an element (for example, <item> ).
	ElementNode
	// TextNode is the text content of a node.
	TextNode
	// CharDataNode node <![CDATA[content]]>
	CharDataNode
	// CommentNode a comment (for example, <!-- my comment --> ).
	CommentNode
	// AttributeNode is an attribute of element.
	AttributeNode
)

type Attr struct {
	Name         xml.Name
	Value        string
	NamespaceURI string
}

// A Node consists of a NodeType and some Data (tag name for
// element nodes, content for text) and are part of a tree of Nodes.
type Node struct {
	Parent, FirstChild, LastChild, PrevSibling, NextSibling *Node

	Type         NodeType
	Data         string
	Prefix       string
	NamespaceURI string
	Attr         []Attr

	level int // node level in the tree
}

// InnerText returns the text between the start and end tags of the object.
func (n *Node) InnerText() string {
	var output func(*bytes.Buffer, *Node)
	output = func(buf *bytes.Buffer, n *Node) {
		switch n.Type {
		case TextNode, CharDataNode:
			buf.WriteString(n.Data)
		case CommentNode:
		default:
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				output(buf, child)
			}
		}
	}

	var buf bytes.Buffer
	output(&buf, n)
	return buf.String()
}

func (n *Node) sanitizedData(preserveSpaces bool) string {
	if preserveSpaces {
		return n.Data
	}
	return strings.TrimSpace(n.Data)
}

func calculatePreserveSpaces(n *Node, pastValue bool) bool {
	if attr := n.SelectAttr("xml:space"); attr == "preserve" {
		return true
	} else if attr == "default" {
		return false
	}
	return pastValue
}

func outputXML(buf *bytes.Buffer, n *Node, preserveSpaces bool) {
	preserveSpaces = calculatePreserveSpaces(n, preserveSpaces)
	switch n.Type {
	case TextNode:
		buf.WriteString(html.EscapeString(n.sanitizedData(preserveSpaces)))
		return
	case CharDataNode:
		buf.WriteString("<![CDATA[")
		buf.WriteString(n.Data)
		buf.WriteString("]]>")
		return
	case CommentNode:
		buf.WriteString("<!--")
		buf.WriteString(n.Data)
		buf.WriteString("-->")
		return
	case DeclarationNode:
		buf.WriteString("<?" + n.Data)
	default:
		if n.Prefix == "" {
			buf.WriteString("<" + n.Data)
		} else {
			buf.WriteString("<" + n.Prefix + ":" + n.Data)
		}
	}

	for _, attr := range n.Attr {
		if attr.Name.Space != "" {
			buf.WriteString(fmt.Sprintf(` %s:%s=`, attr.Name.Space, attr.Name.Local))
		} else {
			buf.WriteString(fmt.Sprintf(` %s=`, attr.Name.Local))
		}
		buf.WriteByte('"')
		buf.WriteString(html.EscapeString(attr.Value))
		buf.WriteByte('"')
	}
	if n.Type == DeclarationNode {
		buf.WriteString("?>")
	} else {
		buf.WriteString(">")
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		outputXML(buf, child, preserveSpaces)
	}
	if n.Type != DeclarationNode {
		if n.Prefix == "" {
			buf.WriteString(fmt.Sprintf("</%s>", n.Data))
		} else {
			buf.WriteString(fmt.Sprintf("</%s:%s>", n.Prefix, n.Data))
		}
	}
}

// OutputXML returns the text that including tags name.
func (n *Node) OutputXML(self bool) string {
	preserveSpaces := calculatePreserveSpaces(n, false)
	var buf bytes.Buffer
	if self && n.Type != DocumentNode {
		outputXML(&buf, n, preserveSpaces)
	} else {
		for n := n.FirstChild; n != nil; n = n.NextSibling {
			outputXML(&buf, n, preserveSpaces)
		}
	}

	return buf.String()
}

// AddAttr adds a new attribute specified by 'key' and 'val' to a node 'n'.
func AddAttr(n *Node, key, val string) {
	var attr Attr
	if i := strings.Index(key, ":"); i > 0 {
		attr = Attr{
			Name:  xml.Name{Space: key[:i], Local: key[i+1:]},
			Value: val,
		}
	} else {
		attr = Attr{
			Name:  xml.Name{Local: key},
			Value: val,
		}
	}

	n.Attr = append(n.Attr, attr)
}

// SetAttr allows an attribute value with the specified name to be changed.
// If the attribute did not previously exist, it will be created.
func (n *Node) SetAttr(key, value string) {
	if i := strings.Index(key, ":"); i > 0 {
		space := key[:i]
		local := key[i+1:]
		for idx := 0; idx < len(n.Attr); idx++ {
			if n.Attr[idx].Name.Space == space && n.Attr[idx].Name.Local == local {
				n.Attr[idx].Value = value
				return
			}
		}

		AddAttr(n, key, value)
	} else {
		for idx := 0; idx < len(n.Attr); idx++ {
			if n.Attr[idx].Name.Local == key {
				n.Attr[idx].Value = value
				return
			}
		}

		AddAttr(n, key, value)
	}
}

// RemoveAttr removes the attribute with the specified name.
func (n *Node) RemoveAttr(key string) {
	removeIdx := -1
	if i := strings.Index(key, ":"); i > 0 {
		space := key[:i]
		local := key[i+1:]
		for idx := 0; idx < len(n.Attr); idx++ {
			if n.Attr[idx].Name.Space == space && n.Attr[idx].Name.Local == local {
				removeIdx = idx
			}
		}
	} else {
		for idx := 0; idx < len(n.Attr); idx++ {
			if n.Attr[idx].Name.Local == key {
				removeIdx = idx
			}
		}
	}
	if removeIdx != -1 {
		n.Attr = append(n.Attr[:removeIdx], n.Attr[removeIdx+1:]...)
	}
}

// AddChild adds a new node 'n' to a node 'parent' as its last child.
func AddChild(parent, n *Node) {
	n.Parent = parent
	n.NextSibling = nil
	if parent.FirstChild == nil {
		parent.FirstChild = n
		n.PrevSibling = nil
	} else {
		parent.LastChild.NextSibling = n
		n.PrevSibling = parent.LastChild
	}

	parent.LastChild = n
}

// AddSibling adds a new node 'n' as a sibling of a given node 'sibling'.
// Note it is not necessarily true that the new node 'n' would be added
// immediately after 'sibling'. If 'sibling' isn't the last child of its
// parent, then the new node 'n' will be added at the end of the sibling
// chain of their parent.
func AddSibling(sibling, n *Node) {
	for t := sibling.NextSibling; t != nil; t = t.NextSibling {
		sibling = t
	}
	n.Parent = sibling.Parent
	sibling.NextSibling = n
	n.PrevSibling = sibling
	n.NextSibling = nil
	if sibling.Parent != nil {
		sibling.Parent.LastChild = n
	}
}

// RemoveFromTree removes a node and its subtree from the document
// tree it is in. If the node is the root of the tree, then it's no-op.
func RemoveFromTree(n *Node) {
	if n.Parent == nil {
		return
	}
	if n.Parent.FirstChild == n {
		if n.Parent.LastChild == n {
			n.Parent.FirstChild = nil
			n.Parent.LastChild = nil
		} else {
			n.Parent.FirstChild = n.NextSibling
			n.NextSibling.PrevSibling = nil
		}
	} else {
		if n.Parent.LastChild == n {
			n.Parent.LastChild = n.PrevSibling
			n.PrevSibling.NextSibling = nil
		} else {
			n.PrevSibling.NextSibling = n.NextSibling
			n.NextSibling.PrevSibling = n.PrevSibling
		}
	}
	n.Parent = nil
	n.PrevSibling = nil
	n.NextSibling = nil
}
