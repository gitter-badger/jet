// Copyright 2016 José Santos <henrique_1609@me.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jet

import (
	"bytes"
	"fmt"
)

var textFormat = "%s" // Changed to "%q" in tests for better error messages.

type Node interface {
	Type() NodeType
	String() string
	Position() Pos
	line() int
	error(error)
	errorf(string, ...interface{})
}

type Expression interface {
	Node
}

// Pos represents a byte position in the original input text from which
// this template was parsed.
type Pos int

func (p Pos) Position() Pos {
	return p
}

// NodeType identifies the type of a parse tree node.
type NodeType int

type NodeBase struct {
	TemplateName string
	Line         int
	NodeType
	Pos
}

func (node *NodeBase) line() int {
	return node.Line
}

func (node *NodeBase) error(err error) {
	node.errorf("%s", err)
}

func (node *NodeBase) errorf(format string, v ...interface{}) {
	panic(fmt.Errorf("Error executing template %q on line %d: %s", node.TemplateName, node.Line, fmt.Sprintf(format, v...)))
}

// Type returns itself and provides an easy default implementation
// for embedding in a Node. Embedded in all non-trivial Nodes.
func (t NodeType) Type() NodeType {
	return t
}

const (
	NodeText       NodeType = iota // Plain text.
	NodeAction                     // A non-control action such as a field evaluation.
	NodeBool                       // A boolean constant.
	NodeChain                      // A sequence of field accesses.
	NodeCommand                    // An element of a pipeline.
	nodeElse                       // An else action. Not added to tree.
	nodeEnd                        // An end action. Not added to tree.
	NodeField                      // A field or method name.
	NodeIdentifier                 // An identifier; always a function name.
	NodeIf                         // An if action.
	NodeList                       // A list of Nodes.
	NodeNil                        // An untyped nil constant.
	NodeNumber                     // A numerical constant.
	NodePipe                       // A pipeline of commands.
	NodeRange                      // A range action.
	NodeString                     // A string constant.
	//NodeWith                       // A with action.
	NodeBlock
	NodeInclude
	NodeYield
	NodeSet

	NodeAdditiveExpr
	NodeMultiplicativeExpr
	NodeComparativeExpr
	NodeNumericComparativeExpr
	NodeLogicalExpr
	NodeCallExpr
	NodeNotExpr
	NodeIssetExpr
	NodeLenExpr
	NodeTernaryExpr
	NodeIndexExpr
	NodeSliceExpr
)

// Nodes.

// ListNode holds a sequence of nodes.
type ListNode struct {
	NodeBase
	Nodes []Node // The element nodes in lexical order.
}

func (l *ListNode) append(n Node) {
	l.Nodes = append(l.Nodes, n)
}

func (l *ListNode) String() string {
	b := new(bytes.Buffer)
	for _, n := range l.Nodes {
		fmt.Fprint(b, n)
	}
	return b.String()
}

// TextNode holds plain text.
type TextNode struct {
	NodeBase
	Text []byte
}

func (t *TextNode) String() string {
	return fmt.Sprintf(textFormat, t.Text)
}

// PipeNode holds a pipeline with optional declaration
type PipeNode struct {
	NodeBase                // The line number in the input. Deprecated: Kept for compatibility.
	Cmds     []*CommandNode // The commands in lexical order.
}

func (p *PipeNode) append(command *CommandNode) {
	p.Cmds = append(p.Cmds, command)
}

func (p *PipeNode) String() string {
	s := ""
	for i, c := range p.Cmds {
		if i > 0 {
			s += " | "
		}
		s += c.String()
	}
	return s
}

// ActionNode holds an action (something bounded by delimiters).
// Control actions have their own nodes; ActionNode represents simple
// ones such as field evaluations and parenthesized pipelines.
type ActionNode struct {
	NodeBase
	Set  *SetNode
	Pipe *PipeNode
}

func (a *ActionNode) String() string {
	if a.Set != nil {
		return fmt.Sprintf("{{%s;%s}}", a.Set, a.Pipe)
	}
	return fmt.Sprintf("{{%s}}", a.Pipe)
}

// CommandNode holds a command (a pipeline inside an evaluating action).
type CommandNode struct {
	NodeBase
	Call     bool
	BaseExpr Expression
	Args     []Expression
}

func (c *CommandNode) append(arg Node) {
	c.Args = append(c.Args, arg)
}

func (c *CommandNode) String() string {
	s := c.BaseExpr.String()
	if c.Call {
		s += ":"
	}
	for i, arg := range c.Args {
		if i > 0 {
			s += ", "
		}
		if _, ok := arg.(*PipeNode); ok {
			s += "(" + arg.String() + ")"
		} else {
			s += arg.String()
		}

	}
	return s
}

// IdentifierNode holds an identifier.
type IdentifierNode struct {
	NodeBase
	Ident string // The identifier's name.
}

func (i *IdentifierNode) String() string {
	return i.Ident
}

// NilNode holds the special identifier 'nil' representing an untyped nil constant.
type NilNode struct {
	NodeBase
}

func (n *NilNode) String() string {
	return "nil"
}

// FieldNode holds a field (identifier starting with '.').
// The names may be chained ('.x.y').
// The period is dropped from each ident.
type FieldNode struct {
	NodeBase
	Ident []string // The identifiers in lexical order.
}

func (f *FieldNode) String() string {
	s := ""
	for _, id := range f.Ident {
		s += "." + id
	}
	return s
}

// ChainNode holds a term followed by a chain of field accesses (identifier starting with '.').
// The names may be chained ('.x.y').
// The periods are dropped from each ident.
type ChainNode struct {
	NodeBase
	Node  Node
	Field []string // The identifiers in lexical order.
}

// Add adds the named field (which should start with a period) to the end of the chain.
func (c *ChainNode) Add(field string) {
	if len(field) == 0 || field[0] != '.' {
		panic("no dot in field")
	}
	field = field[1:] // Remove leading dot.
	if field == "" {
		panic("empty field")
	}
	c.Field = append(c.Field, field)
}

func (c *ChainNode) String() string {
	s := c.Node.String()
	if _, ok := c.Node.(*PipeNode); ok {
		s = "(" + s + ")"
	}
	for _, field := range c.Field {
		s += "." + field
	}
	return s
}

// BoolNode holds a boolean constant.
type BoolNode struct {
	NodeBase
	True bool // The value of the boolean constant.
}

func (b *BoolNode) String() string {
	if b.True {
		return "true"
	}
	return "false"
}

// NumberNode holds a number: signed or unsigned integer, float, or complex.
// The value is parsed and stored under all the types that can represent the value.
// This simulates in a small amount of code the behavior of Go's ideal constants.
type NumberNode struct {
	NodeBase

	IsInt      bool       // Number has an integral value.
	IsUint     bool       // Number has an unsigned integral value.
	IsFloat    bool       // Number has a floating-point value.
	IsComplex  bool       // Number is complex.
	Int64      int64      // The signed integer value.
	Uint64     uint64     // The unsigned integer value.
	Float64    float64    // The floating-point value.
	Complex128 complex128 // The complex value.
	Text       string     // The original textual representation from the input.
}

// simplifyComplex pulls out any other types that are represented by the complex number.
// These all require that the imaginary part be zero.
func (n *NumberNode) simplifyComplex() {
	n.IsFloat = imag(n.Complex128) == 0
	if n.IsFloat {
		n.Float64 = real(n.Complex128)
		n.IsInt = float64(int64(n.Float64)) == n.Float64
		if n.IsInt {
			n.Int64 = int64(n.Float64)
		}
		n.IsUint = float64(uint64(n.Float64)) == n.Float64
		if n.IsUint {
			n.Uint64 = uint64(n.Float64)
		}
	}
}

func (n *NumberNode) String() string {
	return n.Text
}

// StringNode holds a string constant. The value has been "unquoted".
type StringNode struct {
	NodeBase

	Quoted string // The original text of the string, with quotes.
	Text   string // The string, after quote processing.
}

func (s *StringNode) String() string {
	return s.Quoted
}

// endNode represents an {{end}} action.
// It does not appear in the final parse tree.
type endNode struct {
	NodeBase
}

func (e *endNode) String() string {
	return "{{end}}"
}

// elseNode represents an {{else}} action. Does not appear in the final tree.
type elseNode struct {
	NodeBase // The line number in the input. Deprecated: Kept for compatibility.
}

func (e *elseNode) String() string {
	return "{{else}}"
}

// SetNode represents a set action, ident( ',' ident)* '=' expression ( ',' expression )*
type SetNode struct {
	NodeBase
	Let   bool
	Left  []Expression
	Right []Expression
}

func (set *SetNode) String() string {
	var s = ""

	for i, v := range set.Left {
		if i > 0 {
			s += ", "
		}
		s += v.String()
	}

	if set.Let {
		s += ":="
	} else {
		s += "="
	}

	for i, v := range set.Right {
		if i > 0 {
			s += ", "
		}
		s += v.String()
	}

	return s
}

// BranchNode is the common representation of if, range, and with.
type BranchNode struct {
	NodeBase
	Set        *SetNode
	Expression Expression
	List       *ListNode
	ElseList   *ListNode
}

func (b *BranchNode) String() string {

	if b.NodeType == NodeRange {
		s := ""
		if b.Set != nil {
			s = b.Set.String()
		} else {
			s = b.Expression.String()
		}

		if b.ElseList != nil {
			return fmt.Sprintf("{{range %s}}%s{{else}}%s{{end}}", s, b.List, b.ElseList)
		}
		return fmt.Sprintf("{{range %s}}%s{{end}}", s, b.List)
	} else {
		s := ""
		if b.Set != nil {
			s = b.Set.String() + ";"
		}
		if b.ElseList != nil {
			return fmt.Sprintf("{{if %s%s}}%s{{else}}%s{{end}}", s, b.Expression, b.List, b.ElseList)
		}
		return fmt.Sprintf("{{if %s%s}}%s{{end}}", s, b.Expression, b.List)
	}
}

// IfNode represents an {{if}} action and its commands.
type IfNode struct {
	BranchNode
}

// RangeNode represents a {{range}} action and its commands.
type RangeNode struct {
	BranchNode
}

// BlockNode represents a {{block }} action.
type BlockNode struct {
	NodeBase              // The line number in the input. Deprecated: Kept for compatibility.
	Name       string     // The name of the template (unquoted).
	Expression Expression // The command to evaluate as dot for the template.
	List       *ListNode
}

func (t *BlockNode) String() string {
	if t.Expression == nil {
		return fmt.Sprintf("{{block %s}}%s{{end}}", t.Name, t.List)
	}
	return fmt.Sprintf("{{block %s %s}}%s{{end}}", t.Name, t.Expression, t.List)
}

// YieldNode represents a {{yield}} action
type YieldNode struct {
	NodeBase              // The line number in the input. Deprecated: Kept for compatibility.
	Name       string     // The name of the template (unquoted).
	Expression Expression // The command to evaluate as dot for the template.
}

func (t *YieldNode) String() string {
	if t.Expression == nil {
		return fmt.Sprintf("{{yield %s}}", t.Name)
	}
	return fmt.Sprintf("{{yield %s %s}}", t.Name, t.Expression)
}

// IncludeNode represents a {{include }} action.
type IncludeNode struct {
	NodeBase
	Name       string
	Expression Expression
}

func (t *IncludeNode) String() string {
	if t.Expression == nil {
		return fmt.Sprintf("{{include %q}}", t.Name)
	}
	return fmt.Sprintf("{{include %q %s}}", t.Name, t.Expression)
}

type binaryExprNode struct {
	NodeBase
	Operator    item
	Left, Right Expression
}

func (node *binaryExprNode) String() string {
	return fmt.Sprintf("%s %s %s", node.Left, node.Operator.val, node.Right)
}

// AdditiveExprNode represents an add or subtract expression
// ex: expression ( '+' | '-' ) expression
type AdditiveExprNode struct {
	binaryExprNode
}

// MultiplicativeExprNode represents a multiplication, division, or module expression
// ex: expression ( '*' | '/' | '%' ) expression
type MultiplicativeExprNode struct {
	binaryExprNode
}

// LogicalExprNode represents a boolean expression, 'and' or 'or'
// ex: expression ( '&&' | '||' ) expression
type LogicalExprNode struct {
	binaryExprNode
}

// ComparativeExprNode represents a comparative expression
// ex: expression ( '==' | '!=' ) expression
type ComparativeExprNode struct {
	binaryExprNode
}

// NumericComparativeExprNode represents a numeric comparative expression
// ex: expression ( '<' | '>' | '<=' | '>=' ) expression
type NumericComparativeExprNode struct {
	binaryExprNode
}

// NotExprNode represents a negate expression
// ex: '!' expression
type NotExprNode struct {
	NodeBase
	Expr Expression
}

func (s *NotExprNode) String() string {
	return fmt.Sprintf("!%s", s.Expr)
}

// CallExprNode represents a call expression
// ex: expression '(' (expression (',' expression)* )? ')'
type CallExprNode struct {
	NodeBase
	BaseExpr Expression
	Args     []Expression
}

func (s *CallExprNode) String() string {
	arguments := ""
	for i, expr := range s.Args {
		if i > 0 {
			arguments += ", "
		}
		arguments += expr.String()
	}
	return fmt.Sprintf("%s(%s)", s.BaseExpr, arguments)
}

// ex: builtinToken '(' (expression (',' expression)* )? ')'
type BuiltinExprNode struct {
	NodeBase
	Name string
	Args []Expression
}

func (s *BuiltinExprNode) String() string {
	arguments := ""
	for i, expr := range s.Args {
		if i > 0 {
			arguments += ", "
		}
		arguments += expr.String()
	}
	return fmt.Sprintf("%s(%s)", s.Name, arguments)
}

// TernaryExprNod represents a ternary expression,
// ex: expression '?' expression ':' expression
type TernaryExprNode struct {
	NodeBase
	Boolean, Left, Right Expression
}

func (s *TernaryExprNode) String() string {
	return fmt.Sprintf("%s?%s:%s", s.Boolean, s.Left, s.Right)
}

type IndexExprNode struct {
	NodeBase
	Base  Expression
	Index Expression
}

func (s *IndexExprNode) String() string {
	return fmt.Sprintf("%s[%s]", s.Base, s.Index)
}

type SliceExprNode struct {
	NodeBase
	Base     Expression
	Index    Expression
	EndIndex Expression
}

func (s *SliceExprNode) String() string {
	var index_string, len_string string
	if s.Index != nil {
		index_string = s.Index.String()
	}
	if s.EndIndex != nil {
		len_string = s.EndIndex.String()
	}
	return fmt.Sprintf("%s[%s:%s]", s.Base, index_string, len_string)
}
