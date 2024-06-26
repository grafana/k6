// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ast

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

// FileInfo contains information about the contents of a source file, including
// details about comments and items. A lexer accumulates these details as it
// scans the file contents. This allows efficient representation of things like
// source positions.
type FileInfo struct {
	// The name of the source file.
	name string
	// The raw contents of the source file.
	data []byte
	// The offsets for each line in the file. The value is the zero-based byte
	// offset for a given line. The line is given by its index. So the value at
	// index 0 is the offset for the first line (which is always zero). The
	// value at index 1 is the offset at which the second line begins. Etc.
	lines []int
	// The info for every comment in the file. This is empty if the file has no
	// comments. The first entry corresponds to the first comment in the file,
	// and so on.
	comments []commentInfo
	// The info for every lexed item in the file. The last item in the slice
	// corresponds to the EOF, so every file (even an empty one) should have at
	// least one entry. This includes all terminal symbols (tokens) in the AST
	// as well as all comments.
	items []itemSpan
}

type commentInfo struct {
	// the index of the item, in the file's items slice, that represents this
	// comment
	index int
	// the index of the token to which this comment is attributed.
	attributedToIndex int
}

type itemSpan struct {
	// the offset into the file of the first character of an item.
	offset int
	// the length of the item
	length int
}

// NewFileInfo creates a new instance for the given file.
func NewFileInfo(filename string, contents []byte) *FileInfo {
	return &FileInfo{
		name:  filename,
		data:  contents,
		lines: []int{0},
	}
}

func (f *FileInfo) Name() string {
	return f.name
}

// AddLine adds the offset representing the beginning of the "next" line in the file.
// The first line always starts at offset 0, the second line starts at offset-of-newline-char+1.
func (f *FileInfo) AddLine(offset int) {
	if offset < 0 {
		panic(fmt.Sprintf("invalid offset: %d must not be negative", offset))
	}
	if offset > len(f.data) {
		panic(fmt.Sprintf("invalid offset: %d is greater than file size %d", offset, len(f.data)))
	}

	if len(f.lines) > 0 {
		lastOffset := f.lines[len(f.lines)-1]
		if offset <= lastOffset {
			panic(fmt.Sprintf("invalid offset: %d is not greater than previously observed line offset %d", offset, lastOffset))
		}
	}

	f.lines = append(f.lines, offset)
}

// AddToken adds info about a token at the given location to this file. It
// returns a value that allows access to all of the token's details.
func (f *FileInfo) AddToken(offset, length int) Token {
	if offset < 0 {
		panic(fmt.Sprintf("invalid offset: %d must not be negative", offset))
	}
	if length < 0 {
		panic(fmt.Sprintf("invalid length: %d must not be negative", length))
	}
	if offset+length > len(f.data) {
		panic(fmt.Sprintf("invalid offset+length: %d is greater than file size %d", offset+length, len(f.data)))
	}

	tokenID := len(f.items)
	if len(f.items) > 0 {
		lastToken := f.items[tokenID-1]
		lastEnd := lastToken.offset + lastToken.length - 1
		if offset <= lastEnd {
			panic(fmt.Sprintf("invalid offset: %d is not greater than previously observed token end %d", offset, lastEnd))
		}
	}

	f.items = append(f.items, itemSpan{offset: offset, length: length})
	return Token(tokenID)
}

// AddComment adds info about a comment to this file. Comments must first be
// added as items via f.AddToken(). The given comment argument is the Token
// from that step. The given attributedTo argument indicates another token in the
// file with which the comment is associated. If comment's offset is before that
// of attributedTo, then this is a leading comment. Otherwise, it is a trailing
// comment.
func (f *FileInfo) AddComment(comment, attributedTo Token) Comment {
	if len(f.comments) > 0 {
		lastComment := f.comments[len(f.comments)-1]
		if int(comment) <= lastComment.index {
			panic(fmt.Sprintf("invalid index: %d is not greater than previously observed comment index %d", comment, lastComment.index))
		}
		if int(attributedTo) < lastComment.attributedToIndex {
			panic(fmt.Sprintf("invalid attribution: %d is not greater than previously observed comment attribution index %d", attributedTo, lastComment.attributedToIndex))
		}
	}

	f.comments = append(f.comments, commentInfo{index: int(comment), attributedToIndex: int(attributedTo)})
	return Comment{
		fileInfo: f,
		index:    len(f.comments) - 1,
	}
}

// NodeInfo returns details from the original source for the given AST node.
//
// If the given n is out of range, this returns an invalid NodeInfo (i.e.
// nodeInfo.IsValid() returns false). If the given n is not out of range but
// also from a different file than f, then the result is undefined.
func (f *FileInfo) NodeInfo(n Node) NodeInfo {
	return f.nodeInfo(int(n.Start()), int(n.End()))
}

// TokenInfo returns details from the original source for the given token.
//
// If the given t is out of range, this returns an invalid NodeInfo (i.e.
// nodeInfo.IsValid() returns false). If the given t is not out of range but
// also from a different file than f, then the result is undefined.
func (f *FileInfo) TokenInfo(t Token) NodeInfo {
	return f.nodeInfo(int(t), int(t))
}

func (f *FileInfo) nodeInfo(start, end int) NodeInfo {
	if start < 0 || start >= len(f.items) {
		return NodeInfo{fileInfo: f}
	}
	if end < 0 || end >= len(f.items) {
		return NodeInfo{fileInfo: f}
	}
	return NodeInfo{fileInfo: f, startIndex: start, endIndex: end}
}

// ItemInfo returns details from the original source for the given item.
//
// If the given i is out of range, this returns nil. If the given i is not
// out of range but also from a different file than f, then the result is
// undefined.
func (f *FileInfo) ItemInfo(i Item) ItemInfo {
	tok, cmt := f.GetItem(i)
	if tok != TokenError {
		return f.TokenInfo(tok)
	}
	if cmt.IsValid() {
		return cmt
	}
	return nil
}

// GetItem returns the token or comment represented by the given item. Only one
// of the return values will be valid. If the item is a token then the returned
// comment will be a zero value and thus invalid (i.e. comment.IsValid() returns
// false). If the item is a comment then the returned token will be TokenError.
//
// If the given i is out of range, this returns (TokenError, Comment{}). If the
// given i is not out of range but also from a different file than f, then
// the result is undefined.
func (f *FileInfo) GetItem(i Item) (Token, Comment) {
	if i < 0 || int(i) >= len(f.items) {
		return TokenError, Comment{}
	}
	if !f.isComment(i) {
		return Token(i), Comment{}
	}
	// It's a comment, so find its location in f.comments
	c := sort.Search(len(f.comments), func(c int) bool {
		return f.comments[c].index >= int(i)
	})
	if c < len(f.comments) && f.comments[c].index == int(i) {
		return TokenError, Comment{fileInfo: f, index: c}
	}
	// f.isComment(i) returned true, but we couldn't find it
	// in f.comments? Uh oh... that shouldn't be possible.
	return TokenError, Comment{}
}

func (f *FileInfo) isDummyFile() bool {
	return f == nil || f.lines == nil
}

// Sequence represents a navigable sequence of elements.
type Sequence[T any] interface {
	// First returns the first element in the sequence. The bool return
	// is false if this sequence contains no elements. For example, an
	// empty file has no items or tokens.
	First() (T, bool)
	// Next returns the next element in the sequence that comes after
	// the given element. The bool returns is false if there is no next
	// item (i.e. the given element is the last one). It also returns
	// false if the given element is invalid.
	Next(T) (T, bool)
	// Last returns the last element in the sequence. The bool return
	// is false if this sequence contains no elements. For example, an
	// empty file has no items or tokens.
	Last() (T, bool)
	// Previous returns the previous element in the sequence that comes
	// before the given element. The bool returns is false if there is no
	// previous item (i.e. the given element is the first one). It also
	// returns false if the given element is invalid.
	Previous(T) (T, bool)
}

func (f *FileInfo) Items() Sequence[Item] {
	return items{fileInfo: f}
}

func (f *FileInfo) Tokens() Sequence[Token] {
	return tokens{fileInfo: f}
}

type items struct {
	fileInfo *FileInfo
}

func (i items) First() (Item, bool) {
	if len(i.fileInfo.items) == 0 {
		return 0, false
	}
	return 0, true
}

func (i items) Next(item Item) (Item, bool) {
	if item < 0 || int(item) >= len(i.fileInfo.items)-1 {
		return 0, false
	}
	return i.fileInfo.itemForward(item+1, true)
}

func (i items) Last() (Item, bool) {
	if len(i.fileInfo.items) == 0 {
		return 0, false
	}
	return Item(len(i.fileInfo.items) - 1), true
}

func (i items) Previous(item Item) (Item, bool) {
	if item <= 0 || int(item) >= len(i.fileInfo.items) {
		return 0, false
	}
	return i.fileInfo.itemBackward(item-1, true)
}

type tokens struct {
	fileInfo *FileInfo
}

func (t tokens) First() (Token, bool) {
	i, ok := t.fileInfo.itemForward(0, false)
	return Token(i), ok
}

func (t tokens) Next(tok Token) (Token, bool) {
	if tok < 0 || int(tok) >= len(t.fileInfo.items)-1 {
		return 0, false
	}
	i, ok := t.fileInfo.itemForward(Item(tok+1), false)
	return Token(i), ok
}

func (t tokens) Last() (Token, bool) {
	i, ok := t.fileInfo.itemBackward(Item(len(t.fileInfo.items))-1, false)
	return Token(i), ok
}

func (t tokens) Previous(tok Token) (Token, bool) {
	if tok <= 0 || int(tok) >= len(t.fileInfo.items) {
		return 0, false
	}
	i, ok := t.fileInfo.itemBackward(Item(tok-1), false)
	return Token(i), ok
}

func (f *FileInfo) itemForward(i Item, allowComment bool) (Item, bool) {
	end := Item(len(f.items))
	for i < end {
		if allowComment || !f.isComment(i) {
			return i, true
		}
		i++
	}
	return 0, false
}

func (f *FileInfo) itemBackward(i Item, allowComment bool) (Item, bool) {
	for i >= 0 {
		if allowComment || !f.isComment(i) {
			return i, true
		}
		i--
	}
	return 0, false
}

// isComment is comment returns true if i refers to a comment.
// (If it returns false, i refers to a token.)
func (f *FileInfo) isComment(i Item) bool {
	item := f.items[i]
	if item.length < 2 {
		return false
	}
	// see if item text starts with "//" or "/*"
	if f.data[item.offset] != '/' {
		return false
	}
	c := f.data[item.offset+1]
	return c == '/' || c == '*'
}

func (f *FileInfo) SourcePos(offset int) SourcePos {
	lineNumber := sort.Search(len(f.lines), func(n int) bool {
		return f.lines[n] > offset
	})

	// If it weren't for tabs and multibyte unicode characters, we
	// could trivially compute the column just based on offset and the
	// starting offset of lineNumber :(
	// Wish this were more efficient... that would require also storing
	// computed line+column information, which would triple the size of
	// f's items slice...
	col := 0
	for i := f.lines[lineNumber-1]; i < offset; i++ {
		if f.data[i] == '\t' {
			nextTabStop := 8 - (col % 8)
			col += nextTabStop
		} else if utf8.RuneStart(f.data[i]) {
			col++
		}
	}

	return SourcePos{
		Filename: f.name,
		Offset:   offset,
		Line:     lineNumber,
		// Columns are 1-indexed in this AST
		Col: col + 1,
	}
}

// Token represents a single lexed token.
type Token int

// TokenError indicates an invalid token. It is returned from query
// functions when no valid token satisfies the request.
const TokenError = Token(-1)

// AsItem returns the Item that corresponds to t.
func (t Token) AsItem() Item {
	return Item(t)
}

func (t Token) asTerminalNode() terminalNode {
	return terminalNode(t)
}

// Item represents an item lexed from source. It represents either
// a Token or a Comment.
type Item int

// ItemInfo provides details about an item's location in the source file and
// its contents.
type ItemInfo interface {
	SourceSpan
	LeadingWhitespace() string
	RawText() string
}

// NodeInfo represents the details for a node or token in the source file's AST.
// It provides access to information about the node's location in the source
// file. It also provides access to the original text in the source file (with
// all the original formatting intact) and also provides access to surrounding
// comments.
type NodeInfo struct {
	fileInfo             *FileInfo
	startIndex, endIndex int
}

var _ ItemInfo = NodeInfo{}

// IsValid returns true if this node info is valid. If n is a zero-value struct,
// it is not valid.
func (n NodeInfo) IsValid() bool {
	return n.fileInfo != nil
}

// Start returns the starting position of the element. This is the first
// character of the node or token.
func (n NodeInfo) Start() SourcePos {
	if n.fileInfo.isDummyFile() {
		return UnknownPos(n.fileInfo.name)
	}

	tok := n.fileInfo.items[n.startIndex]
	return n.fileInfo.SourcePos(tok.offset)
}

// End returns the ending position of the element, exclusive. This is the
// location after the last character of the node or token. If n returns
// the same position for Start() and End(), the element in source had a
// length of zero (which should only happen for the special EOF token
// that designates the end of the file).
func (n NodeInfo) End() SourcePos {
	if n.fileInfo.isDummyFile() {
		return UnknownPos(n.fileInfo.name)
	}

	tok := n.fileInfo.items[n.endIndex]
	// find offset of last character in the span
	offset := tok.offset
	if tok.length > 0 {
		offset += tok.length - 1
	}
	pos := n.fileInfo.SourcePos(offset)
	if tok.length > 0 {
		// We return "open range", so end is the position *after* the
		// last character in the span. So we adjust
		pos.Col++
	}
	return pos
}

// LeadingWhitespace returns any whitespace prior to the element. If there
// were comments in between this element and the previous one, this will
// return the whitespace between the last such comment in the element. If
// there were no such comments, this returns the whitespace between the
// previous element and the current one.
func (n NodeInfo) LeadingWhitespace() string {
	if n.fileInfo.isDummyFile() {
		return ""
	}

	tok := n.fileInfo.items[n.startIndex]
	var prevEnd int
	if n.startIndex > 0 {
		prevTok := n.fileInfo.items[n.startIndex-1]
		prevEnd = prevTok.offset + prevTok.length
	}
	return string(n.fileInfo.data[prevEnd:tok.offset])
}

// LeadingComments returns all comments in the source that exist between the
// element and the previous element, except for any trailing comment on the
// previous element.
func (n NodeInfo) LeadingComments() Comments {
	if n.fileInfo.isDummyFile() {
		return EmptyComments
	}

	start := sort.Search(len(n.fileInfo.comments), func(i int) bool {
		return n.fileInfo.comments[i].attributedToIndex >= n.startIndex
	})

	if start == len(n.fileInfo.comments) || n.fileInfo.comments[start].attributedToIndex != n.startIndex {
		// no comments associated with this token
		return EmptyComments
	}

	numComments := 0
	for i := start; i < len(n.fileInfo.comments); i++ {
		comment := n.fileInfo.comments[i]
		if comment.attributedToIndex == n.startIndex &&
			comment.index < n.startIndex {
			numComments++
		} else {
			break
		}
	}

	return Comments{
		fileInfo: n.fileInfo,
		first:    start,
		num:      numComments,
	}
}

// TrailingComments returns the trailing comment for the element, if any.
// An element will have a trailing comment only if it is the last token
// on a line and is followed by a comment on the same line. Typically, the
// following comment is a line-style comment (starting with "//").
//
// If the following comment is a block-style comment that spans multiple
// lines, and the next token is on the same line as the end of the comment,
// the comment is NOT considered a trailing comment.
//
// Examples:
//
//	foo // this is a trailing comment for foo
//
//	bar /* this is a trailing comment for bar */
//
//	baz /* this is a trailing
//	       comment for baz */
//
//	fizz /* this is NOT a trailing
//	        comment for fizz because
//	        its on the same line as the
//	        following token buzz */       buzz
func (n NodeInfo) TrailingComments() Comments {
	if n.fileInfo.isDummyFile() {
		return EmptyComments
	}

	start := sort.Search(len(n.fileInfo.comments), func(i int) bool {
		comment := n.fileInfo.comments[i]
		return comment.attributedToIndex >= n.endIndex &&
			comment.index > n.endIndex
	})

	if start == len(n.fileInfo.comments) || n.fileInfo.comments[start].attributedToIndex != n.endIndex {
		// no comments associated with this token
		return EmptyComments
	}

	numComments := 0
	for i := start; i < len(n.fileInfo.comments); i++ {
		comment := n.fileInfo.comments[i]
		if comment.attributedToIndex == n.endIndex {
			numComments++
		} else {
			break
		}
	}

	return Comments{
		fileInfo: n.fileInfo,
		first:    start,
		num:      numComments,
	}
}

// RawText returns the actual text in the source file that corresponds to the
// element. If the element is a node in the AST that encompasses multiple
// items (like an entire declaration), the full text of all items is returned
// including any interior whitespace and comments.
func (n NodeInfo) RawText() string {
	startTok := n.fileInfo.items[n.startIndex]
	endTok := n.fileInfo.items[n.endIndex]
	return string(n.fileInfo.data[startTok.offset : endTok.offset+endTok.length])
}

// SourcePos identifies a location in a proto source file.
type SourcePos struct {
	Filename string
	// The line and column numbers for this position. These are
	// one-based, so the first line and column is 1 (not zero). If
	// either is zero, then the line and column are unknown and
	// only the file name is known.
	Line, Col int
	// The offset, in bytes, from the beginning of the file. This
	// is zero-based: the first character in the file is offset zero.
	Offset int
}

func (pos SourcePos) String() string {
	if pos.Line <= 0 || pos.Col <= 0 {
		return pos.Filename
	}
	return fmt.Sprintf("%s:%d:%d", pos.Filename, pos.Line, pos.Col)
}

// SourceSpan represents a range of source positions.
type SourceSpan interface {
	Start() SourcePos
	End() SourcePos
}

// NewSourceSpan creates a new span that covers the given range.
func NewSourceSpan(start SourcePos, end SourcePos) SourceSpan {
	return sourceSpan{StartPos: start, EndPos: end}
}

type sourceSpan struct {
	StartPos SourcePos
	EndPos   SourcePos
}

func (p sourceSpan) Start() SourcePos {
	return p.StartPos
}

func (p sourceSpan) End() SourcePos {
	return p.EndPos
}

var _ SourceSpan = sourceSpan{}

// Comments represents a range of sequential comments in a source file
// (e.g. no interleaving items or AST nodes).
type Comments struct {
	fileInfo   *FileInfo
	first, num int
}

// EmptyComments is an empty set of comments.
var EmptyComments = Comments{}

// Len returns the number of comments in c.
func (c Comments) Len() int {
	return c.num
}

func (c Comments) Index(i int) Comment {
	if i < 0 || i >= c.num {
		panic(fmt.Sprintf("index %d out of range (len = %d)", i, c.num))
	}
	return Comment{
		fileInfo: c.fileInfo,
		index:    c.first + i,
	}
}

// Comment represents a single comment in a source file. It indicates
// the position of the comment and its contents. A single comment means
// one line-style comment ("//" to end of line) or one block comment
// ("/*" through "*/"). If a longer comment uses multiple line comments,
// each line is considered to be a separate comment. For example:
//
//	// This is a single comment, and
//	// this is a separate comment.
type Comment struct {
	fileInfo *FileInfo
	index    int
}

var _ ItemInfo = Comment{}

// IsValid returns true if this comment is valid. If this comment is
// a zero-value struct, it is not valid.
func (c Comment) IsValid() bool {
	return c.fileInfo != nil && c.index >= 0
}

// AsItem returns the Item that corresponds to c.
func (c Comment) AsItem() Item {
	return Item(c.fileInfo.comments[c.index].index)
}

func (c Comment) Start() SourcePos {
	span := c.fileInfo.items[c.AsItem()]
	return c.fileInfo.SourcePos(span.offset)
}

func (c Comment) End() SourcePos {
	span := c.fileInfo.items[c.AsItem()]
	return c.fileInfo.SourcePos(span.offset + span.length - 1)
}

func (c Comment) LeadingWhitespace() string {
	item := c.AsItem()
	span := c.fileInfo.items[item]
	var prevEnd int
	if item > 0 {
		prevItem := c.fileInfo.items[item-1]
		prevEnd = prevItem.offset + prevItem.length
	}
	return string(c.fileInfo.data[prevEnd:span.offset])
}

func (c Comment) RawText() string {
	span := c.fileInfo.items[c.AsItem()]
	return string(c.fileInfo.data[span.offset : span.offset+span.length])
}
