package lexer

import "monkey/token"

type Lexer struct {
	input        string // goのコード
	position     int    // 入力における現在の位置（現在の文字を指し示す）
	readPosition int    // これから読み込む位置（現在の文字の次）
	ch           byte   // 現愛検査中の文字
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	// spaceは無視する。
	// これがあるかないかでspaceに意味を持たせるか持たせないかが決まる。
	l.skipWhitespace()

	switch l.ch {
	case '=':
		// = は単体でも使えるし、 == と使われることもある。
		// そのため = が現れたら次の文字を覗き見して == であるかどうかを判定する。
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar() // 次の文字が = だったので、 == としてTokenを用意するためにポジションを読み進める。
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.EQ, Literal: literal}
		} else {
			tok = newToken(token.ASSIGN, l.ch)
		}
	case '+':
		tok = newToken(token.PLUS, l.ch)
	case '-':
		tok = newToken(token.MINUS, l.ch)
	case '!':
		// ! は単体でも使えるし、 != と使われることもある。
		// そのため ! が現れたら次の文字を覗き見して != であるかどうかを判定する。
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar() // 次の文字が = だったので、 != としてTokenを用意するためにポジションを読み進める。
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NOT_EQ, Literal: literal}
		} else {
			tok = newToken(token.BANG, l.ch)
		}
	case '/':
		tok = newToken(token.SLASH, l.ch)
	case '*':
		tok = newToken(token.ASTERISK, l.ch)
	case '<':
		tok = newToken(token.LT, l.ch)
	case '>':
		tok = newToken(token.GT, l.ch)
	case ';':
		tok = newToken(token.SEMICOLON, l.ch)
	case ',':
		tok = newToken(token.COMMA, l.ch)
	case '{':
		tok = newToken(token.LBRACE, l.ch)
	case '}':
		tok = newToken(token.RBRACE, l.ch)
	case '(':
		tok = newToken(token.LPAREN, l.ch)
	case ')':
		tok = newToken(token.RPAREN, l.ch)
	// 文字列リテラル
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
	// 配列リテラル
	case '[':
		tok = newToken(token.LBRACKET, l.ch)
	case ']':
		tok = newToken(token.RBRACKET, l.ch)
	// ハッシュリテラルのなかで使う
	case ':':
		tok = newToken(token.COLON, l.ch)
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
	default:
		// 英字だったら
		if isLetter(l.ch) {
			// 英字で有る限り、バイトを読み進める。
			tok.Literal = l.readIdentifier()
			// 読み進めた一塊の英字が予約語かどうか判定。
			// 予約語だったら、予約語のTokenType、不明な英字ならユーザー定義の文字列のTokenType（IDENT）を返す
			tok.Type = token.LookupIdent(tok.Literal)
			// ここで即returnをしているのはreadIdentifierのなかで、すでにreadPositionを進めているから。
			// switchの後のl.readChar()を呼ぶ必要がない。
			return tok
			// 数値だったら
		} else if isDigit(l.ch) {
			tok.Type = token.INT
			// 数値で有る限り、バイトを読み進める。
			tok.Literal = l.readNumber()
			// ここで即returnをしているのはreadNumberのなかで、すでにreadPositionを進めているから。
			// switchの後のl.readChar()を呼ぶ必要がない。
			return tok
			// 英字でも数値でもなければ、不明のTokenTypeを返す
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}
	}

	// readPositionを次に進めておく。
	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readChar() {
	// inputはgoのコード。inputを読み切ったら終端まで達成したことになるのでl.chを0にする。
	// l.chが0 だと NextToken()でEOFのトークンが生成される
	// 	case 0:
	//		tok.Literal = ""
	//		tok.Type = token.EOF
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		// l.chを次のバイトに読み進める。つまり、1バイトで一つの文字という制約がある。
		// マルチバイト文字には対応していない。
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition += 1 // readPositionを次のバイトを指すようにする。
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// 現在の文字が " （文字列リテラルの終端） か 0 (EOF) に達するまで、一つのSTRINGトークンとして読み進める
func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.position]
}

// 次の文字を覗き見するための関数。
// 「覗き見」するだけなので、position, readPositionを進めることはしない。
func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	} else {
		return l.input[l.readPosition]
	}
}

// letter（英字）
func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

// 数値を整数としてしか判定しない。不動小数点や16進数、8進数などはサポート外。
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// chには各トークンタイプごとに読み進め終わったbyteがやってくる。
func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}
