package xmlquery

import (
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
	index float32
}

type outputConfiguration struct {
	printSelf              bool
	preserveSpaces         bool
	emptyElementTagSupport bool
	skipComments           bool
}

type OutputOption func(*outputConfiguration)

// WithOutputSelf configures the Node to print the root node itself
func WithOutputSelf() OutputOption {
	return func(oc *outputConfiguration) {
		oc.printSelf = true
	}
}

// WithEmptyTagSupport empty tags should be written as <empty/> and
// not as <empty></empty>
func WithEmptyTagSupport() OutputOption {
	return func(oc *outputConfiguration) {
		oc.emptyElementTagSupport = true
	}
}

// WithoutComments will skip comments in output
func WithoutComments() OutputOption {
	return func(oc *outputConfiguration) {
		oc.skipComments = true
	}
}

// Index returns the index of node.
func (n *Node) Index() float32 {
	switch n.Type {
	case AttributeNode:
		return n.Parent.index
	case TextNode, CharDataNode, CommentNode:
		if n.PrevSibling == nil {
			return n.Parent.index
		}
		searchCount := 1
		nn := n.PrevSibling
		for {
			switch nn.Type {
			case TextNode, CharDataNode, CommentNode:
				if nn.PrevSibling == nil {
					nn = nn.PrevSibling
					searchCount += 1
					continue
				}
				return nn.Parent.index + float32(searchCount)/1000.0
			default:
				return nn.index + float32(searchCount)/1000.0
			}
		}
	}
	return n.index
}

// InnerText returns the text between the start and end tags of the object.
func (n *Node) InnerText() string {
	var output func(*strings.Builder, *Node)
	output = func(b *strings.Builder, n *Node) {
		switch n.Type {
		case TextNode, CharDataNode:
			b.WriteString(n.Data)
		case CommentNode:
		default:
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				output(b, child)
			}
		}
	}

	var b strings.Builder
	output(&b, n)
	return b.String()
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

func outputXML(b *strings.Builder, n *Node, preserveSpaces bool, config *outputConfiguration) {
	preserveSpaces = calculatePreserveSpaces(n, preserveSpaces)
	switch n.Type {
	case TextNode:
		b.WriteString(html.EscapeString(n.sanitizedData(preserveSpaces)))
		return
	case CharDataNode:
		b.WriteString("<![CDATA[")
		b.WriteString(n.Data)
		b.WriteString("]]>")
		return
	case CommentNode:
		if !config.skipComments {
			b.WriteString("<!--")
			b.WriteString(n.Data)
			b.WriteString("-->")
		}
		return
	case DeclarationNode:
		b.WriteString("<?" + n.Data)
	default:
		if n.Prefix == "" {
			b.WriteString("<" + n.Data)
		} else {
			b.WriteString("<" + n.Prefix + ":" + n.Data)
		}
	}

	for _, attr := range n.Attr {
		if attr.Name.Space != "" {
			b.WriteString(fmt.Sprintf(` %s:%s=`, attr.Name.Space, attr.Name.Local))
		} else {
			b.WriteString(fmt.Sprintf(` %s=`, attr.Name.Local))
		}
		b.WriteByte('"')
		b.WriteString(html.EscapeString(attr.Value))
		b.WriteByte('"')
	}
	if n.Type == DeclarationNode {
		b.WriteString("?>")
	} else {
		if n.FirstChild != nil || !config.emptyElementTagSupport {
			b.WriteString(">")
		} else {
			b.WriteString("/>")
			return
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		outputXML(b, child, preserveSpaces, config)
	}
	if n.Type != DeclarationNode {
		if n.Prefix == "" {
			b.WriteString(fmt.Sprintf("</%s>", n.Data))
		} else {
			b.WriteString(fmt.Sprintf("</%s:%s>", n.Prefix, n.Data))
		}
	}
}

// OutputXML returns the text that including tags name.
func (n *Node) OutputXML(self bool) string {

	config := &outputConfiguration{
		printSelf:              true,
		emptyElementTagSupport: false,
	}
	preserveSpaces := calculatePreserveSpaces(n, false)
	var b strings.Builder
	if self && n.Type != DocumentNode {
		outputXML(&b, n, preserveSpaces, config)
	} else {
		for n := n.FirstChild; n != nil; n = n.NextSibling {
			outputXML(&b, n, preserveSpaces, config)
		}
	}

	return b.String()
}

// OutputXMLWithOptions returns the text that including tags name.
func (n *Node) OutputXMLWithOptions(opts ...OutputOption) string {

	config := &outputConfiguration{}
	// Set the options
	for _, opt := range opts {
		opt(config)
	}

	preserveSpaces := calculatePreserveSpaces(n, false)
	var b strings.Builder
	if config.printSelf && n.Type != DocumentNode {
		outputXML(&b, n, preserveSpaces, config)
	} else {
		for n := n.FirstChild; n != nil; n = n.NextSibling {
			outputXML(&b, n, preserveSpaces, config)
		}
	}

	return b.String()
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
