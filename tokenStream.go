package govaluate

import "sync"

type tokenStream struct {
	tokens      []ExpressionToken
	index       int
	tokenLength int
	budget      *budgetTracker
}

var tokenStreamPool = sync.Pool{
	New: func() interface{} {
		return new(tokenStream)
	},
}

func newTokenStream(tokens []ExpressionToken) *tokenStream {
	ret := tokenStreamPool.Get().(*tokenStream)
	ret.tokens = tokens
	ret.index = 0
	ret.tokenLength = len(tokens)
	ret.budget = nil
	return ret
}

/*
enterDepth records descent into a nested grammatical construct (parenthesized clause,
function arguments, or a chained operator) against the stream's budget, if any.
*/
func (t *tokenStream) enterDepth() error {

	return t.budget.enter()
}

/*
exitDepth records ascent from a nested grammatical construct.
It must be paired with a successful call to enterDepth.
*/
func (t *tokenStream) exitDepth() {

	t.budget.exit()
}

func (t *tokenStream) rewind() {
	t.index -= 1
}

func (t *tokenStream) next() ExpressionToken {
	token := t.tokens[t.index]

	t.index += 1
	return token
}

func (t tokenStream) hasNext() bool {

	return t.index < t.tokenLength
}

func (t *tokenStream) close() {
	tokenStreamPool.Put(t)
}
