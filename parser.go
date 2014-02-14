package php

import (
	"fmt"
	"stephensearles.com/php/ast"
)

type parser struct {
	lexer *lexer

	previous []item
	idx      int
	current  item
	errors   []error

	parenLevel int
}

func newParser(input string) *parser {
	p := &parser{
		idx:   -1,
		lexer: newLexer(input),
	}
	return p
}

func (p *parser) parse() []ast.Node {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(p.errors)
			fmt.Println(r)
		}
	}()
	// expecting either itemHTML or itemPHPBegin
	nodes := make([]ast.Node, 0, 1)
TokenLoop:
	for {
		p.next()
		switch p.current.typ {
		case itemEOF:
			break TokenLoop
		default:
			n := p.parseNode()
			if n != nil {
				nodes = append(nodes, n)
			}
		}
	}
	return nodes
}

func (p *parser) parseNode() ast.Node {
	switch p.current.typ {
	case itemHTML:
		return ast.EchoStmt(ast.Literal{ast.String})
	case itemPHPBegin:
		return nil
	case itemPHPEnd:
		return nil
	}
	return p.parseStmt()
}

func (p *parser) next() {
	p.idx += 1
	if len(p.previous) <= p.idx {
		p.current = p.lexer.nextItem()
		p.previous = append(p.previous, p.current)
	} else {
		p.current = p.previous[p.idx]
	}
}

func (p *parser) backup() {
	p.idx -= 1
	p.current = p.previous[p.idx]
}

func (p *parser) expect(i itemType) {
	p.next()
	if p.current.typ != i {
		p.expected(i)
	}
}

func (p *parser) expected(i itemType) {
	p.errorf("Found %s, expected %s", p.current, i)
}

func (p *parser) errorf(str string, args ...interface{}) {
	p.errors = append(p.errors, fmt.Errorf(str, args...))
	if len(p.errors) > 0 {
		panic("too many errors")
	}
}

func (p *parser) parseIf() *ast.IfStmt {
	p.expect(itemOpenParen)
	n := &ast.IfStmt{}
	p.next()
	n.Condition = p.parseExpression()
	p.expect(itemCloseParen)
	p.next()
	n.TrueBlock = p.parseStmt()
	p.next()
	if p.current.typ == itemElse {
		p.next()
		n.FalseBlock = p.parseStmt()
	} else {
		n.FalseBlock = ast.Block{}
		p.backup()
	}
	return n
}

func (p *parser) parseExpression() (expr ast.Expression) {
	expr = ast.UnknownTypeExpression{}
TypeLoop:
	for ; ; p.next() {
		switch p.current.typ {
		case itemStringLiteral:
			expr = ast.Literal{ast.String}
		case itemNumberLiteral:
			expr = ast.Literal{ast.Float}
		case itemTrueLiteral:
			expr = ast.Literal{ast.Boolean}
		case itemFalseLiteral:
			expr = ast.Literal{ast.Boolean}
		case itemOperator:
		case itemIdentifier:
		case itemOpenParen:
			p.parenLevel += 1
		case itemCloseParen:
			if p.parenLevel == 0 {
				break TypeLoop
			}
			p.parenLevel -= 1
		case itemNonVariableIdentifier:
			return p.parseFunctionCall()
		default:
			break TypeLoop
		}
	}
	p.backup()
	return expr
}

func (p *parser) parseFunctionCall() ast.FunctionCallExpression {
	expr := ast.FunctionCallExpression{}
	if p.current.typ != itemNonVariableIdentifier {
		p.expected(itemNonVariableIdentifier)
	}
	expr.FunctionName = p.current.val
	expr.Arguments = make([]ast.Expression, 0)
	p.expect(itemOpenParen)
	first := true
	for {
		p.next()
		if p.current.typ == itemCloseParen {
			break
		}
		if !first {
			p.expect(itemArgumentSeparator)
		} else {
			first = false
		}
		expr.Arguments = append(expr.Arguments, p.parseExpression())
	}
	return expr
}

func (p *parser) parseStmt() ast.Statement {
	switch p.current.typ {
	case itemBlockBegin:
		return p.parseBlock()
	case itemIdentifier:
		n := ast.AssignmentStmt{}
		n.Assignee = ast.Identifier{p.current.val}
		p.expect(itemOperator)
		p.next()
		n.Value = p.parseExpression()
		p.expect(itemStatementEnd)
		return n
	case itemFunction:
		return p.parseFunctionStmt()
	case itemEcho:
		p.next()
		expr := p.parseExpression()
		p.expect(itemStatementEnd)
		return ast.EchoStmt(expr)
	case itemIf:
		return p.parseIf()
	case itemNonVariableIdentifier:
		stmt := p.parseExpression()
		p.expect(itemStatementEnd)
		return stmt
	default:
		p.errorf("Found %s, expected html or php begin", p.current)
		return nil
	}
}

func (p *parser) parseFunctionStmt() *ast.FunctionStmt {
	stmt := &ast.FunctionStmt{}
	p.expect(itemNonVariableIdentifier)
	stmt.Name = p.current.val
	p.expect(itemOpenParen)
	first := true
	for {
		p.next()
		if p.current.typ == itemCloseParen {
			break
		}
		p.backup()
		if !first {
			p.expect(itemArgumentSeparator)
		} else {
			first = false
		}
		p.expect(itemIdentifier)
	}
	stmt.Body = p.parseBlock()
	return stmt
}

func (p *parser) parseBlock() ast.Block {
	block := ast.Block{}
	p.expect(itemBlockBegin)
	for {
		p.next()
		block.Statements = append(block.Statements, p.parseStmt())
		if p.next(); p.current.typ == itemBlockEnd {
			break
		}
		p.backup()
	}
	return block
}
