package parser

import (
	"monkey/ast"
	"monkey/lexer"
	"monkey/token"
	"fmt"
	"strconv"
)

type Parser struct {
	l *lexer.Lexer

	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, errors: []string{}}

	// 2つのトークンを読み込む
	// curTokenとpeekTokenの両方がセットされる
	p.nextToken()
	p.nextToken()

	// 前置演算子Parser関数を登録
	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)

	// 中置演算子Parserの関数を登録
	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)

	return p
}

// 次のトークンを取得する
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// プログラムを解析する
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for p.curToken.Type != token.EOF {
		stmt := p.parseStatement()
		fmt.Printf("stmd:%s", stmt)
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}
	return program
}

// 式の解析
// トークンによって解析方法を切り替える
func (p *Parser) parseStatement() ast.Statement {
	// カレントトークンによって解読方法を切り替える
	switch p.curToken.Type {
	case token.LET:
		return p.parseLetStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	default:
		return p.parseExpressionStatement()
	}
}

// Let文の解読
func (p *Parser) parseLetStatement() *ast.LetStatement {

	// let statement
	stmt := &ast.LetStatement{Token: p.curToken}

	//
	fmt.Printf("Type:%s,Literal:%s\n", p.curToken.Type, p.curToken.Literal)
	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// 次は"="なはず
	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	// TODO: セミコロンに遭遇するまで式を読み飛ばしてしまっている
	for !p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

// return文の解析
func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	for !p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// カレントトークンのタイプチェック
func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

// Nextトークンのタイプチェック
func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got%s instead", t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

// 次のトークンが想定するものか
func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		p.peekError(t)
		return false
	}
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn func(ast.Expression) ast.Expression
)

// 前置演算子タイプごとの処理を登録
func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// 中置演算子タイプごとの処理を登録
func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// 式の解析
func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {

	defer untrace(trace("parseExpressionStatement"))
	//  statementの生成
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	// Expressionの解析
	stmt.Expression = p.parseExpression(LOWEST)

	// 次のトークンがセミコロンの場合、トークンを飛ばす
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// 式の解析
func (p *Parser) parseExpression(precedence int) ast.Expression {
	defer untrace(trace("parseExpression"))
	// prefixタイプによって解析処理を取得する
	prefix := p.prefixParseFns[p.curToken.Type]
	// 該当するprefixタイプがなければ・・・
	if prefix == nil {
		p.noPrefixParserFnError(p.curToken.Type)
		return nil
	}
	// prefixを
	leftExp := prefix()

	// 次のTOKENが中置演算子かどうかを見る
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

// 数値リテラルの解析 String → Int
func (p *Parser) parseIntegerLiteral() ast.Expression {
	defer untrace(trace("parseIntegerLiteral"))

	// String → Int
	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)

	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	// Literalの生成
	return &ast.IntegerLiteral{Token: p.curToken, Value: value}
}

// 識別子の解析
func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

const (
	_           int = iota
	LOWEST
	EQUALS
	LESSGREATER
	SUM
	PRODUCT
	PREFIX
	CALL
)

// 前置演算式解析エラー
func (p *Parser) noPrefixParserFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

// 前置演算式の解析
func (p *Parser) parsePrefixExpression() ast.Expression {
	defer untrace(trace("parsePrefixExpression"))
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}
// 中置演算式の解析
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parseInfixExpression"))
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)
	return expression
}

// 優先順位テーブル
var precedences = map[token.TokenType]int{
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
}

// 次のTokenの優先順位を取得
func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

// 現在のTokenの優先順位を取得
func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}
