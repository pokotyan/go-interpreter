package evaluator

import (
	"fmt"
	"monkey/ast"
	"monkey/object"
)

// null、true、falseはどのコンテキストでも同じもの。
// 毎回objectを生成する必要はないので、Evalではここのポインタを参照させて返すようにする。
var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

// ASTを辿っていき、評価する。
// 末端のノードであることが確定しているIntegerやBoolなどは自身のノードの値を返す。
// 配下にノードを持つノードの場合(Expressionとか)は、再帰的にEvalを呼び出し続ける。
//
// エラーハンドリングについて
// プログラムを評価する際(Evalを実行した際)、isError() で評価結果が Errorオブジェクトだったかどうかを 必ず 確かめる。
// で、Errorオブジェクトの場合は即returnさせているので、Evalの再帰ループから脱出し、評価が終了する。
//
// envについて
// env は変数への値の束縛に使う。
// envはmap構造になっていて、LetStatementの評価がされるたびに更新されていく。
func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	// --------------
	// Statements（評価の結果、値を返さない）
	// --------------
	case *ast.Program:
		//fmt.Println("Program--------------")
		return evalProgram(node, env)
	case *ast.ExpressionStatement:
		//fmt.Println("ExpressionStatement--------------")
		return Eval(node.Expression, env)
	case *ast.BlockStatement:
		//fmt.Println("BlockStatement--------------")
		return evalBlockStatement(node, env)
	case *ast.ReturnStatement:
		//fmt.Println("ReturnStatement--------------")
		val := Eval(node.ReturnValue, env) // ReturnValueはExpressionなので、Eval内ではExpressionStatementが実行される。
		if isError(val) {
			return val
		}
		// ReturnStatementが来たら、returnの右側の式を評価して、その値を返す。なので、return文の後に何か書いていても評価されない。
		return &object.ReturnValue{Value: val}
	case *ast.LetStatement:
		//fmt.Println("LetStatement--------------")
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.Set(node.Name.Value, val) // 評価結果をletで宣言したIDENTに束縛させる

	// --------------
	// Expressions（評価の結果、値を返す）
	// --------------
	case *ast.IntegerLiteral:
		//fmt.Println("IntegerLiteral--------------")
		return &object.Integer{Value: node.Value}
	case *ast.StringLiteral:
		//fmt.Println("StringLiteral--------------")
		return &object.String{Value: node.Value}
	case *ast.Boolean:
		//fmt.Println("Boolean--------------")
		return nativeBoolToBooleanObject(node.Value)
	case *ast.PrefixExpression: // ! or -
		//fmt.Println("PrefixExpression--------------")
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		//fmt.Println("InfixExpression--------------")
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)
	case *ast.IfExpression:
		//fmt.Println("IfExpression--------------")
		return evalIfExpression(node, env)
	// 変数に束縛された値をenvから確認し、返す。
	// 束縛されている変数が見つからなかった場合は組み込み関数を探し、Builtinオブジェクトを返す。
	case *ast.Identifier:
		//fmt.Println("Identifier--------------")
		return evalIdentifier(node, env)
	// ユーザー定義の関数の関数オブジェクトの生成
	case *ast.FunctionLiteral:
		//fmt.Println("FunctionLiteral--------------")
		params := node.Parameters
		body := node.Body
		// Envには関数を定義した場所のスコープがはいる
		return &object.Function{Parameters: params, Env: env, Body: body}
	// 関数呼び出し
	case *ast.CallExpression:
		//fmt.Println("CallExpression--------------")
		// Functionオブジェクトの取得。ここのEvalの処理は、関数がユーザー定義か、組み組みかの違いにより、再帰の流れが異なってくる。
		// ＜ユーザー定義の関数の場合＞
		//   parseの結果、node.Functionには、FunctionLiteralのExpressionが入っている。
		//   なので、Evalの処理はこのあと、 case *ast.FunctionLiteral: の分岐を辿ることになる。
		//   結果、functionには object.Function が格納される。
		// ＜組み込み関数の場合＞
		//   parseの結果、node.Functionには、IdentifierのExpressionが入っている。
		//   なので、Evalの処理はこのあと、 case *ast.Identifier: の分岐を辿ることになる。
		//   evalIdentifierの処理の中では、組み込み関数が存在するIDENTの場合、*object.Builtin を返すようになっている。
		//   結果、functionには object.Builtin が格納される。
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}

		args := evalExpressions(node.Arguments, env) // 引数郡（評価済み）を取得。
		// evalExpressionsの処理内ではArgumentsのいずれかでエラーが発生するとそのエラーのみが返ってくる。でそのエラーを返す。
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}

		// functionはユーザー定義の関数(object.Function)の場合と、組み込み関数の場合(object.Builtin)がある。
		// applyFunctionのなかでどちらなのか確認し処理をする。
		return applyFunction(function, args)
	case *ast.ArrayLiteral:
		//fmt.Println("ArrayLiteral--------------")
		elements := evalExpressions(node.Elements, env)
		// evalExpressionsの処理内ではElementsのいずれかでエラーが発生するとそのエラーのみが返ってくる。でそのエラーを返す。
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}
	// 添字アクセス。添字アクセスは配列とハッシュがある。
	case *ast.IndexExpression:
		//fmt.Println("IndexExpression--------------")
		// 添字の対象になる式を評価する。
		// ・配列の場合
		// 　Leftの式は最終的に、Evalの case *ast.ArrayLiteral: の分岐を経て object.Array になり、leftに入る。
		// ・ハッシュの場合
		// 　Leftの式は最終的に、Evalの case *ast.HashLiteral: の分岐を経て object.Hash になり、leftに入る。
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}

		// 添字の式を評価する。
		// ・配列の場合
		// 　添字の式は最終的に、Evalの case *ast.IntegerLiteral: の分岐を経て object.Integer になりindexに入る。
		//   object.Integerにならない式の場合、evalIndexExpressionの処理内でエラーになる。
		// ・ハッシュの場合
		// 　添字の式は評価した結果、Hashableインタフェースを満たすオブジェクトであればOK。
		//   Hashableインタフェースを満たさないものだった場合、evalIndexExpression から呼び出される evalHashIndexExpression の処理でエラーになる。
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)
	case *ast.HashLiteral:
		//fmt.Println("HashLiteral--------------")
		return evalHashLiteral(node, env)
	}

	return nil
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

// 前置演算子で ! が現れたら 右側の 式 の結果を反転させる
func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		// true、false、nullのオブジェクト以外はここにくる。integerとか。
		// ex: !5
		// rightに5(integer)がある場合はfalseになる。つまり、 !5 は false として扱う = integerはtruthyなものとして扱う設計。
		// ex: !!5
		// !!5 の場合は、(!(!5)) と解釈される。「木の深いところからEvalの結果が出る」ので、まず
		// (!5) はfalse => (!false) はtrue、となるので !!5はtrueと解釈される。
		// ex: !!-5
		// (!(!(-5))) と解釈される。
		// -5 で一括りの式。（*ast.PrefixExpression）。これはここのcaseにくるので、!(-5) は false。で、さらに ! があるので反転してtrue。
		// なので、!!-5 はtrue。
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	// - の前置演算子を置けるのは、右側がintegerの時だけ。
	// このルールに反してたらエラー
	if right.Type() != object.INTEGER_OBJ {
		return newError("unknown operator: -%s", right.Type())
	}

	value := right.(*object.Integer).Value
	return &object.Integer{Value: -value} // 整数のprefixに - をつけたIntegerオブジェクトを返す
}

func evalInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	switch {
	// 二項演算の左右が数値なら
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		// 四則演算 or 比較の評価をする
		return evalIntegerInfixExpression(operator, left, right)
	// 文字列結合なら
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	// boolの比較 ex: true == true
	case operator == "==":
		// TRUE、FALSEのオブジェクトはポインタ。（つどオブジェクト生成はしていない）なのでここではポインタ同士の比較をしている。
		return nativeBoolToBooleanObject(left == right)
	// boolの比較 ex: !false != false
	case operator == "!=":
		// TRUE、FALSEのオブジェクトはポインタ。（つどオブジェクト生成はしていない）なのでここではポインタ同士の比較をしている。
		return nativeBoolToBooleanObject(left != right)
	// 同じジャンルのオブジェクトじゃないと、二項演算はできない。IDENTならIDENT同士で演算する。IDENTとINTでは演算できない設計
	case left.Type() != right.Type():
		return newError("type mismatch: %s %s %s",
			left.Type(), operator, right.Type())
	// 上記に当てはまらない場合はエラー
	default:
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalIntegerInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		return &object.Integer{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalStringInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	// 文字列は + の結合のみサポートする。文字列同士の引き算や ==、!= の比較などは対応していない。
	if operator != "+" {
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}

	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value
	return &object.String{Value: leftVal + rightVal}
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}

	return result
}

func evalBlockStatement(
	block *ast.BlockStatement,
	env *object.Environment,
) object.Object {
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		// block内でReturnValueオブジェクトがあったらそのオブジェクトを返す。returnの式を評価した値はここでは返さない。
		// なぜかというと、以下のようなネストしたblockを考える。
		// if (10 > 1) {
		// 	 if (10 > 1) {
		// 		return 10;
		// 	 }
		//
		//	 return 1;
		// }
		// このプログラムは1を返すべきなプログラム。
		// このプログラムの return 10 で ReturnValue.Value をもし返してしまった場合、10はIntegerなので、次のEvalで処理されるのは
		// case *ast.IntegerLiteral:
		//	 return &object.Integer{Value: node.Value}
		// の部分となり、Eval関数の再帰的な処理が終わってしまう。上記のプログラムが返す値は本来 1 のはずだが、10となってしまう。
		// この evalBlockStatement 関数内で ReturnValueオブジェクトが現れた際、
		// オブジェクトをアンラップせずにそのまま返すことでEvalの再帰処理を止めることがなくなるので、ネストしたブロックでもちゃんと評価できるようになる。
		//
		// あとは、評価の結果が Error オブジェクトだった時もそれを結果として返す必要がある。
		// block内の返り値となりうる値は returnした値 か 発生したエラー なので、
		// if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ { という条件になる。
		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}

	return result
}

// if (<condition>) <consequence> else <alternative>
func evalIfExpression(
	ie *ast.IfExpression,
	env *object.Environment,
) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func evalIdentifier(
	node *ast.Identifier,
	env *object.Environment,
) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}

	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}

	return newError("identifier not found: " + node.Value)
}

// 関数の引数郡と配列内の要素の評価
func evalExpressions(
	exps []ast.Expression,
	env *object.Environment,
) []object.Object {
	var result []object.Object

	// 引数は左から順に評価される。
	for _, e := range exps {
		evaluated := Eval(e, env)
		// 各要素のいずれかでerrorが発生しようものなら、後続の要素の評価はせず、発生したエラーのみを返す。
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch fn := fn.(type) {
	// ユーザー定義の関数なら
	case *object.Function:
		// 関数が実行される時は、現在の環境で評価するのではなく、Functionオブジェクトが持っているEnvで評価する。
		// Functionオブジェクトが持っているEnvは、その関数が定義された時の環境への参照。
		// まとめると関数は「自身が定義された環境で評価する」
		extendedEnv := extendFunctionEnv(fn, args) // 関数定義時の環境と引数の束縛をマージしたenvを作る
		evaluated := Eval(fn.Body, extendedEnv)    // 現在の環境ではなく、関数が持っている環境で評価する
		return unwrapReturnValue(evaluated)
	// 組み組み関数なら
	case *object.Builtin:
		return fn.Fn(args...)
	default:
		return newError("not a function: %s", fn.Type())
	}
}

// ここら辺のenvのコードがクロージャを実現している。
// クロージャのところ、ややこしいからわからなくなったら、167ページを確認
func extendFunctionEnv(
	fn *object.Function,
	args []object.Object,
) *object.Environment {
	// fn.Envは関数を定義した場所のスコープが入っている。そのスコープを外側とする内側のスコープをここで作っている。
	// ここで作られたenvは outer に、「関数を定義した場所のスコープ(fn.env)」を持つ。
	// で、env.Getは内側から外側(outer)のscopeを再帰的に確認するので、ここで作成しているenvは「関数を定義した場所のスコープ」にアクセスできるenv。
	// つまり、関数を呼び出すと
	// ・envの層が内側に一枚増える。（現在のenvを外側として、内側に層が増える）
	// ・呼び出された関数内では自身が定義された環境のスコープにアクセス可能
	// これでクロージャが実現できる（理解があってるかは不安）
	env := object.NewEnclosedEnvironment(fn.Env)

	// 引数の値をenvに入れる。
	// これで、
	// 外側(outer)のenv: 関数を定義した際の環境
	// 内側のenv: 引数の値
	// という情報を持つenvが作られる。
	// このenvの束縛情報を元にBlockStatementのEvalが実行されることで、関数が実行される。
	for paramIdx, param := range fn.Parameters {
		env.Set(param.Value, args[paramIdx])
	}

	return env
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(left, index)
	default:
		return newError("index operator not supported: %s", left.Type())
	}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	// 存在しない添字アクセスはNULLを返す
	if idx < 0 || idx > max {
		return NULL
	}

	return arrayObject.Elements[idx] // goの添字機能を使って添字アクセスを評価する。
}

func evalHashLiteral(
	node *ast.HashLiteral,
	env *object.Environment,
) object.Object {
	pairs := make(map[object.HashKey]object.HashPair)

	// Pairsのmapにはキー、バリュー共にexpressionノードが入っている。
	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env) // expressionをEvalし、String、Boolean、Integerオブジェクトのいずれかが生成される
		if isError(key) {
			return key
		}

		// ハッシュのキーになれるオブジェクトはHashableインタフェースを満たす
		// String、Boolean、IntegerオブジェクトはいずれもHashableインタフェースを満たしている。
		hashKey, ok := key.(object.Hashable)
		if !ok {
			return newError("unusable as hash key: %s", key.Type())
		}

		value := Eval(valueNode, env) // valueのexpressionノードをEvalし、式の評価結果をvalueに入れる。
		if isError(value) {
			return value
		}

		// object.Hash.PairsのmapのキーはHashKey構造体を入れる。
		hashed := hashKey.HashKey()
		pairs[hashed] = object.HashPair{Key: key, Value: value}
	}

	return &object.Hash{Pairs: pairs}
}

// hashからindexで指定した添字の値を取り出す
func evalHashIndexExpression(hash, index object.Object) object.Object {
	hashObject := hash.(*object.Hash)

	// ハッシュのキーとなれるオブジェクトはHashableインタフェースを満たす必要がある。
	key, ok := index.(object.Hashable)
	if !ok {
		return newError("unusable as hash key: %s", index.Type())
	}

	// indexで指定したキーから導かれるHashKey構造体に一致するバリューをハッシュから取り出す。
	// ハッシュのキーの探索にはHashKey()を使う。
	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}

	return pair.Value
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}

	return obj
}

func isTruthy(obj object.Object) bool {
	// NULLでもTRUEでもFALSEでもなければtruthyな値、という設計。ex: 10はtruthy
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
}

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}
