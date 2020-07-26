package parser

import (
	"fmt"
	"monkey/ast"
	"monkey/lexer"
	"monkey/token"
	"strconv"
)

const (
	_ int = iota
	LOWEST
	EQUALS      // ==
	LESSGREATER // > or <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index]
)

// 優先順位。下に行くほど優先順位高。
var precedences = map[token.TokenType]int{
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.PLUS:     SUM,     // + と、
	token.MINUS:    SUM,     // - は同じ優先順位。
	token.SLASH:    PRODUCT, // 割り算と、
	token.ASTERISK: PRODUCT, // 掛け算は同じ優先順位。かつ、+や-より優先度が高い。
	token.LPAREN:   CALL,    // 関数呼び出し。
	token.LBRACKET: INDEX,   // 配列の添字。関数呼び出しより優先度が高い。add(1 + myArr[1]) という式の場合、 [1] が木の中で一番深い階層になる。
}

type (
	prefixParseFn func() ast.Expression               // 前置
	infixParseFn  func(ast.Expression) ast.Expression // 後置（引数は左側の式）
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	// -----初期処理として全てのトークンの解析関数を登録しておく------
	// 前置（先頭に登場する（いきなり登場する）ことができるtokenたち）
	// BANGとMINUSは前置演算子として使えるので解析関数の中でトークンを一つ進める。
	// そして、MINUSは前置、中置のどちらにもいる。前置演算子としても使えるし、中置演算子としても当然使える。
	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)  // !
	p.registerPrefix(token.MINUS, p.parsePrefixExpression) // -
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression) // (
	p.registerPrefix(token.IF, p.parseIfExpression)
	p.registerPrefix(token.FUNCTION, p.parseFunctionLiteral)
	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral) // [ 配列リテラルの始まり
	p.registerPrefix(token.LBRACE, p.parseHashLiteral)    // { ハッシュリテラルの始まり

	// 中置（前置の後に登場することができるトークンたち）
	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)

	// 関数呼び出しのための ( に対する中置解析関数の登録
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	// 配列の添字 [ のための中置解析関数の登録
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken() // ここでlexerとparserが繋がる
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		// tが期待するトークン出なければエラーをため込む。
		// ここでため込んだエラーはテストの際のチェックに利用する。
		p.peekError(t)
		return false
	}
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead",
		t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for p.curToken.Type != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.LET:
		return p.parseLetStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	default:
		return p.parseExpressionStatement()
	}
}

// let <identifier> = <expression>;
func (p *Parser) parseLetStatement() *ast.LetStatement {
	// まずLETのstatementを用意
	stmt := &ast.LetStatement{Token: p.curToken}

	// 次のトークンがIDENTであれば、トークンを次へ進めた上で、ここはtrueになる
	if !p.expectPeek(token.IDENT) {
		return nil
	}

	// letの後にはユーザー定義のIDENTが来る
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// 次のトークンがASSIGN(=)であること。正しければ = にトークンを進める。
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	// = の次へトークンを進める。（進めた先のトークンはexpressionになる）
	p.nextToken()

	// 式のトークンに紐づけられた解析関数を実行しValueに入れる。
	stmt.Value = p.parseExpression(LOWEST)

	// トークンが;になるまで読み進める。;が省略されていたとしてもエラーにはしない。
	if p.peekTokenIs(token.SEMICOLON) {
		// ; にトークンを移動する。
		p.nextToken()
	}

	return stmt
}

// return <expression>;
func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	// returnの次のexpressionにトークンを進める。
	p.nextToken()

	// returnの右側の式をparseし、ReturnValueに入れる。
	stmt.ReturnValue = p.parseExpression(LOWEST)

	// 次が;なら;にトークンを進める。
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	//defer untrace(trace("parseExpressionStatement"))
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	//defer untrace(trace("parseExpression"))

	// ---------前置演算子の解析---------
	// 現在のトークンに前置解析関数があるか
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		// 解析関数がないとエラー。
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	// 前置解析関数の実行をする
	// これは、式の一発目のトークンがINTやIDENTならcastとかをするだけの解析になる。その処理の中ではトークンを進めたりしない。
	// ↑ parseIdentifier、parseIntegerLiteral
	// もし、式の一発目が ! や - だったら前置演算子として解析が必要。そのため前置演算子の右側の式も一括りに解析する。なので解析処理の中ではトークンを進める。
	// ↑ parsePrefixExpression
	// なので、curTokenがINTやIDENTの場合は前置解析関数というより、currentの解析関数ってイメージの方がしっくりくる。
	leftExp := prefix()

	// ---------中置演算子の再帰的な解析---------
	// 次のトークンが;ではない、かつ
	// 次のトークンの優先順位が、引数で渡されている優先順位より高い間は、（引数で渡される優先順位は初回はLOWEST、以降は式に現れた演算子の優先順位）
	// parseInfixExpressionを繰り返し、ループさせる。
	// 逆に言うと、次のトークンの優先順位がLOWESTになるまでループは続く。
	// 優先順位がLOWESTになるトークンというのは、EOFやIDENTやINTなどのそもそも演算子ではないtoken。
	// つまり、式の終わりになるまで解析を続ける。
	// イメージとしては a * b * c という式があった場合、
	// 1）a は前置解析関数（parseIdentifier）で解析する。現在のトークンは a のまま。
	// 2) a の 次のトークンは * であり、LOWESTより優先度が高いので中置解析関数のループに入る。
	// 3) p.infixParseFns[p.peekToken.Type] で * の解析関数を取得。
	// 4) nextTokenで現在のトークンが * に移動。
	// 5) parseInfixExpressionの実行。ast.InfixExpressionノードを生成する。
	//    ast.InfixExpressionノードはLeftとRightにExpressionをもち、Operatorに演算子を持つ。
	//    LeftのExpressionは前置解析関数の結果(leftExp)を引数で渡しているのでそれを入れる。
	//    Operatorには現在のトークン * を入れる。
	//    RightのExpressionはparseInfixExpressionの中でnextTokenでトークンを進め、parseExpressionをしているのでその結果を入れる。
	//    今回の例で言うとtokenが b に進んだ上でparseExpressionが実行される。引数には b の左の演算子 * の優先順位を渡す。そして2回目のparseExpressionに入る。
	// 6) 2回目のparseExpression。b を前置のparseIdentifierで解析する。そして1回目のparseExpressionと違うところは中置解析関数のループには入らないというところ。
	//    なぜかと言うと precedence < p.peekPrecedence() がfalseになるから。bの左にある * と右にある * の優先順位を比較してるわけだが、これは同じ優先順位なのでfalseになる。
	//    これで、ast.InfixExpressionのLeftとRightにast.IntegerLiteralがぶら下がる木「a * b」ができることになる。
	//    forループに入らないことで、再帰的なparseExpressionの処理を抜けて、「a * b」の木は親のast.InfixExpressionのleftに代入される。
	// 7) 再度forループの判定。現在のトークンはbであり、優先度はLOWEST。bの次の * の方が優先度が高いので2回目のforループに突入する。
	//    1回目のforループ突入と違うところは1回目の時のleftExpは a だけだったが、今回は 「a * b」の木構造がleftExpとして渡される。
	//    curTokenを二個目の * に進めた状態でparseInfixExpressionの処理が実行される。
	//    parseInfixExpressionの処理の中でtokenを次の c に進め、parseExpressionを実行する。
	// 8) ３回目のparseExpression。c の前置解析が実行されast.Identifierが生成される。そしてforループには入らない。
	//    なぜかと言うと 「c（IDENT）の次のトークンはEOF」、「c（IDENT）とEOFの優先度は同じ」なのでforループの条件を満たさない。
	//    これで、3回目のparseExpressionは終わり、呼び出し元のparseInfixExpression処理に戻る。
	//    3回目のparseExpressionの戻り値である、ast.Identifierがast.InfixExpressionのRightに入って処理が終わる。
	// ポイント) ast.InfixExpressionのleftは再帰的
	//    再帰的にparseExpressionを実行し、peekTokenが最後(EOF)まで来たらforループを抜けることができる。
	//    再帰処理の一回目ではast.InfixExpressionのleftに入る値は初回の前置解析の結果。
	//    注目すべきポイントとして、再帰処理が繰り返されるごとに、このleftに入る木は肥大化していく。
	//    処理一回目: 前置解析の結果
	//    処理二回目: 前置解析と中置解析の結果
	//    処理三回目: ((前置解析と中置解析の結果) + 前置解析と中置解析の結果)
	//    こんな感じでast.InfixExpressionのLeftは再帰的にast.InfixExpressionノードが入る木構造になる。イメージはp76の図を参照
	// ポイント2) !p.peekTokenIs(token.SEMICOLON)について
	//    forループの !p.peekTokenIs(token.SEMICOLON) はなくても動く。次のトークンがない場合peekTokenはEOFとなり、次のトークンの優先度はLOWESTになる。
	//    だが、こういう条件をつけることでこの言語は;を式の終端として扱うこともできる、ということがわかりやすくなる。
	// ポイント3) precedence < p.peekPrecedence()について
	//    次の演算子の結合力（左結合力）が現在の結合力（右結合力）より高いかどうか。もしそうであれば今まで構文解析したものは次の演算子に吸い込まれる。積み上げてきた左に右が吸い込まれて、深い木になっていく。
	//    逆に左より右の結合力が高い場合、forループはtrueにならず、ノードはast.InfixExpressionのrightに配置される。
	//    左結合力はpeekPrecedence（次のトークン）の優先度。右結合力はprecedence（現在のトークン）の優先度。
	//    現在のトークンの優先度（右結合力）が次のトークンの優先度（左結合力）を上回れば、左側の再帰的な木構造に吸い込まれることなく、右側のノードになる。
	//    逆であれば、現在のトークンは左側へ吸い込まれていく。
	//    つまり、カッコが現れた時に、precedence（現在のトークン、右結合力）の値をいじることで、左と右のどちらの木構造を深くするか（深いほど、優先度が高い）をハンドリングできる。
	//    これを利用すれば、ユーザー定義の優先度（括弧）に対応することができる。
	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

// 次のトークンの優先順位を確認。なければ最低の優先順位をデフォで返す。
func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}

	return LOWEST
}

// 現在のトークンの優先順位を確認。なければ最低の優先順位をデフォで返す。
func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

// トークンリテラルに文字列で入っている数値をint64に変換し、astノードのvalueに入れるためのヘルパー
func (p *Parser) parseIntegerLiteral() ast.Expression {
	//defer untrace(trace("parseIntegerLiteral"))
	lit := &ast.IntegerLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value

	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

// <prefix operator><expression>
// 前置の演算子である、token.INT、token.BANGの解析と、その右側のexpressionの解析。
func (p *Parser) parsePrefixExpression() ast.Expression {
	//defer untrace(trace("parsePrefixExpression"))
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	// -5のような文字列を *ast.PrefixExpression として一括りに構築するためには
	// 前置演算子の解析後、すぐに次のトークンに移り、トークンを二つ消費する必要がある。そのためにnextTokenする。
	p.nextToken()

	// そして、前置演算子の右側のExpressionをparseする。
	// 前置演算子の優先順位(PREFIX)を引数として渡す
	expression.Right = p.parseExpression(PREFIX)

	return expression
}

// 中置演算子の式のparse。curTokenが中置の演算子にまで進んだ状態で呼ばれる。
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	//defer untrace(trace("parseInfixExpression"))
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()                  // 中置演算子の優先順位の確認。
	p.nextToken()                                    // tokenを右側のexpressionにまで進める。1 + 2 なら 2 にtokenを進める感じ。
	expression.Right = p.parseExpression(precedence) // 右側の式を解析する。引数に中置演算子優先順位を渡す。

	return expression
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function} // ( 関数呼び出しの括弧
	exp.Arguments = p.parseExpressionList(token.RPAREN)               // ) がくるまでカンマ区切りの引数をパースする。
	return exp
}

// 関数呼び出しは3パターン。引数あり、引数一つのみ、引数が複数。
// ()
// (<expression>)
// (<expression>, <expression>, <expression>, ...)
func (p *Parser) parseCallArguments() []ast.Expression {
	args := []ast.Expression{}

	// 引数が何もない場合。( の次のトークンが ) だった場合
	if p.peekTokenIs(token.RPAREN) {
		// ) にトークンを進める。
		p.nextToken()
		return args
	}

	// ------ここから下は引数ありで関数呼び出しをしている場合------
	// ( の次へ（一つ目の引数（式））へトークンを進める。
	p.nextToken()
	// 引数（式）の解析。
	args = append(args, p.parseExpression(LOWEST))

	// 一つ目の引数(式)の次が , だった場合。複数の引数を渡して関数呼び出しをしている場合、このforループに入る。
	for p.peekTokenIs(token.COMMA) {
		// , にトークンを進める。
		p.nextToken()
		// 次の引数(式)にトークンを進める。
		p.nextToken()
		// 次の引数を引数配列に入れる。
		args = append(args, p.parseExpression(LOWEST))
	}

	// 関数呼び出しの終わりは ) であるはず。正しければ ) にトークンを進める。
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return args
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

// ユーザーが書いた括弧の優先度を高くする魔法の関数
// ( が現れたらこの関数が実行される。
// ===================== ex: 1 + (2 + 3) =====================
// 1 + (2 + 3)の場合、
// --------------
// 「(」 LPAREN LOWEST    「2」 INT LOWEST
// precedence          <   p.peekPrecedence()
// --------------
// の比較になり、どちらも優先度はLOWESTなのでinfixのforループに入らなくなる。つまりinfixノードの右側のメンバーになる。
// そして右側のノードにいる状態で、
// --------------
// 「2」 INT LOWEST     「+」 PLUS SUM
// precedence       <   p.peekPrecedence()
// --------------
// の比較が行われ、infixのforループに入り、木の右側が深くなる。
// 結果、1 + (2 + 3) は、(1 + (2 + 3)) になる。
//
// ===================== ex: 1 + 2 + 3 =====================
// 1 + 2 + 3の場合、
// --------------
// 「2」 INT LOWEST     「+」 PLUS SUM
// precedence       <   p.peekPrecedence()
// --------------
// の比較になり、peekPrecedenceの方が大きいため、infixのforループに入る。infixノードの左側のメンバーになる。
// 結果、1 + 2 + 3 は ((1 + 2) + 3) になる。
func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

// if (<condition>) <consequence> else <alternative>
func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	//if の次は ( であること
	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	// ( にトークンを進める。
	p.nextToken()
	// ( をparseGroupedExpressionで解析する。次のトークンが ) になるまでトークンが進む。
	expression.Condition = p.parseExpression(LOWEST)

	// 次のトークンが ) であること。正しければトークンを ) に進める。
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	// 次のトークンが { であること。正しければトークンを { に進める。
	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	// <consequence> の部分の解析。
	expression.Consequence = p.parseBlockStatement()

	// ここに来たときは if (<condition>) <consequence> まで解析が終わっており、現在位置は } にいる。
	// もし次のトークンがelseなら、elseの解析に進む。elseでないなら解析を終える。
	if p.peekTokenIs(token.ELSE) {
		// elseにトークンを進める。
		p.nextToken()

		// else の次は { であること。正しければトークンを { に進める。
		if !p.expectPeek(token.LBRACE) {
			return nil
		}

		// else の ブロックの解析を行う。
		expression.Alternative = p.parseBlockStatement()
	}

	return expression
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	// [ をTokenとしてArrayLiteralのノードを作成
	array := &ast.ArrayLiteral{Token: p.curToken}

	// curTokenが配列の終端である ] になるまで、パースを続ける。
	array.Elements = p.parseExpressionList(token.RBRACKET)

	return array
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	// [ をTokenとしてIndexExpressionのノードを作成
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}

	// 添字の中身にトークンを進める。
	p.nextToken()
	// 添字の中身のexpressionノードをIndexに入れる。
	exp.Index = p.parseExpression(LOWEST)

	// 次のトークンがRBRACKET ] であること。そうであればトークンを次へ進め、ここはtrueになる
	// 添字の終端は ] でないとnilを返す。
	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	// カンマ区切りの要素にトークンを進める。 [ や ( の次にトークンを進める。
	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

	// 要素の一つ目のパースが終わり、次のトークンが , ならこのループに入る。
	// , がある限り、パースし続ける。
	for p.peekTokenIs(token.COMMA) {
		p.nextToken() // , にトークンを進める
		p.nextToken() // 次の配列の要素にトークンを進める
		list = append(list, p.parseExpression(LOWEST))
	}

	// 次のトークンが引数で渡ってきたtokenであること。そうであればトークンを次へ進め、ここはtrueになる
	// 引数で渡ってくるトークンは ) or ] の二通り。
	if !p.expectPeek(end) {
		return nil
	}

	return list
}

// { <expression>:<expression>, <expression>:<expression>, ... }
func (p *Parser) parseHashLiteral() ast.Expression {
	// { をTokenに入れる。
	hash := &ast.HashLiteral{Token: p.curToken}
	// Pairsの初期化。
	hash.Pairs = make(map[ast.Expression]ast.Expression)

	// 次のtokenが } ではない間は、ハッシュの中身をパースし続ける。
	for !p.peekTokenIs(token.RBRACE) {
		p.nextToken()                    // ハッシュの中身にトークンを進める
		key := p.parseExpression(LOWEST) // キーの式をパースする

		// 次のトークンが : なら、トークンを : に進める。（キーの後は : がくるはず）
		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken()                      // バリューにトークンを進める
		value := p.parseExpression(LOWEST) // バリューの式をパースする。

		hash.Pairs[key] = value // パースしたキーバリューをPairsに入れる。goのmapをそのまま利用する。

		// 1組のキーバリューが終わった後は、 } もしくは , がくるはず。
		// そうではない場合は、hashの構文としておかしいのでnilを返す。
		if !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
			return nil
		}
	}

	// 次のtokenが } だったら、トークンを進める。 } 以外だったらnilを返す。
	// hashの終端は } であるはず。
	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return hash
}

// fn <parameters> <block statement>
func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken} // fn トークン

	// fn の次は ( があるはず。正しければトークンを ( に進める。
	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	// 引数の解析
	lit.Parameters = p.parseFunctionParameters()

	// 引数が終われば ) があるはず。正しければトークンを ) に進める。
	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	// ) の次は <block statement> の解析。
	lit.Body = p.parseBlockStatement()

	return lit
}

// 引数の解析。以下の3つのバリエーションに対応する。
// (<IDENT>, <IDENT>, <IDENT>, ...)
// (<IDENT>)
// ()
func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	identifiers := []*ast.Identifier{}

	// 引数が何もない場合。( の次のトークンが ) だった場合
	if p.peekTokenIs(token.RPAREN) {
		// ) にトークンを進める。
		p.nextToken()
		return identifiers
	}

	// -------ここからは引数が一つでもあった場合-------
	// 一つ目の引数(IDENT)にトークンを進める。
	p.nextToken()

	// Identノードを作成
	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	// 冒頭で用意した引数配列に一つ目の引数を詰める。
	identifiers = append(identifiers, ident)

	// 一つ目の引数の後に , が現れた場合。つまり複数の引数がある場合はこのforループに入る。
	for p.peekTokenIs(token.COMMA) {
		// , にトークンを進める。
		p.nextToken()
		// 次の引数にトークンを進める。
		p.nextToken()
		// 次の引数のIdentノードを作成。
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		// 作成したIdentノードを引数配列に詰める
		identifiers = append(identifiers, ident)
	}

	// 引数の終わりには ) があるはず。正しければ ) にトークンを進める。
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return identifiers
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	// ブロックの中にトークンを移動させる。
	p.nextToken()

	// } が出てくる、もしくはEOFが出てくるまではブロックの中を解析し続ける。
	// EOFの時はstmtがnilになり、現在まで解析したものをblock.Statementsにつめて終了？？？（ちょっと自信ない）
	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

// 前置の構文解析関数を登録
func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// 後置の構文解析関数を登録
func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}
