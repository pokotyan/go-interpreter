package object

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"monkey/ast"
	"strings"
)

type BuiltinFunction func(args ...Object) Object
type ObjectType string

const (
	NULL_OBJ  = "NULL"
	ERROR_OBJ = "ERROR"

	INTEGER_OBJ = "INTEGER"
	BOOLEAN_OBJ = "BOOLEAN"
	STRING_OBJ  = "STRING"

	RETURN_VALUE_OBJ = "RETURN_VALUE"

	FUNCTION_OBJ = "FUNCTION"
	BUILTIN_OBJ  = "BUILTIN"

	ARRAY_OBJ = "ARRAY"
	HASH_OBJ  = "HASH"
)

type HashKey struct {
	Type  ObjectType
	Value uint64
}

type Hashable interface {
	HashKey() HashKey
}

type Object interface {
	Type() ObjectType
	Inspect() string
}

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) HashKey() HashKey { // Integerをhashのキーとして使う場合、この関数を用いる
	return HashKey{Type: i.Type(), Value: uint64(i.Value)}
}

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) HashKey() HashKey { // Booleanをhashのキーとして使う場合、この関数を用いる
	var value uint64

	if b.Value {
		value = 1
	} else {
		value = 0
	}

	return HashKey{Type: b.Type(), Value: value}
}

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

// もし字句解析器がエラー発生時、行やカラムの番号をトークンに付与するようになっていれば、ここにはそのプロパティが追加されるだろう
type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

type Function struct {
	Parameters []*ast.Identifier   // 引数
	Body       *ast.BlockStatement // 処理内容
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}

	out.WriteString("fn")
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(f.Body.String())
	out.WriteString("\n}")

	return out.String()
}

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }
func (s *String) HashKey() HashKey { // Stringをhashのキーとして使う場合、この関数を用いる
	h := fnv.New64a()
	h.Write([]byte(s.Value))

	return HashKey{Type: s.Type(), Value: h.Sum64()}
}

type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }

type Array struct {
	Elements []Object
}

func (ao *Array) Type() ObjectType { return ARRAY_OBJ }
func (ao *Array) Inspect() string {
	var out bytes.Buffer

	var elements []string
	for _, e := range ao.Elements {
		elements = append(elements, e.Inspect())
	}

	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")

	return out.String()
}

type HashPair struct {
	Key   Object
	Value Object
}

// PairsのmapのキーはHashKey構造体。
// ハッシュのキーになりうる値は整数、文字列、booleanだが、この3つのオブジェクトはHashKeyメソッドを持つようにしている。
// （なぜこういう作りにしているのか、なぜキーにオブジェクトをそのまま格納しないのか、はobject_test.goを参照）
//
// PairsのmapのバリューはHashPair構造体。
// なぜオブジェクトをそのまま格納する作りではなく、キーペアの構造体を入れるようにしているのか答えよう。
// これはInspectでキーバリューの情報を両方表示させたいため。Inspectでバリューのみが表示されるのでは使い勝手が悪いから。
// これにより、REPLでハッシュを表示する際、キーバリューの両方が表示される
//
// なのでHash.Pairsはキーもバリューも構造体になっているmap。
// 実際に評価する際はこんな感じのコードになる。
// pairs[hashKey.HashKey()] = object.HashPair{Key: key, Value: value}
type Hash struct {
	Pairs map[HashKey]HashPair
}

func (h *Hash) Type() ObjectType { return HASH_OBJ }
func (h *Hash) Inspect() string {
	var out bytes.Buffer

	var pairs []string
	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s",
			pair.Key.Inspect(), pair.Value.Inspect()))
	}

	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}
