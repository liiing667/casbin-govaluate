package govaluate

import (
	"errors"
	"fmt"
	"time"
)

/*
Budget describes resource limits applied while parsing and evaluating a single expression.
It is intended for expressions built from untrusted input: malformed expressions with extreme
nesting or very long operator chains can otherwise consume unbounded stack and memory.

The zero Budget imposes no limits and matches the historical behavior of this library.
*/
type Budget struct {

	/*
		MaxTokens caps how many tokens a single expression may contain.
		Zero means no limit.
	*/
	MaxTokens int

	/*
		MaxDepth caps the structural depth of an expression. Each level of parenthesis nesting,
		function argument list, prefix operator, or chained (same-precedence) operator consumes
		one level of depth. Zero means no limit.
	*/
	MaxDepth int

	/*
		Timeout caps the wall-clock time spent in a single parse or a single evaluation.
		Each parsing constructor call and each Eval call gets its own fresh timeout window.
		The check is best-effort: it is sampled periodically, and cannot preempt a single
		long-running operation (such as a user-defined function). Zero means no limit.
	*/
	Timeout time.Duration
}

/*
Errors returned when a Budget is exceeded. They are delivered as ordinary error returns,
never panics, and can be tested with errors.Is.
*/
var (
	ErrTokenBudgetExceeded   = errors.New("expression exceeds token budget")
	ErrDepthBudgetExceeded   = errors.New("expression exceeds depth budget")
	ErrTimeoutBudgetExceeded = errors.New("expression exceeds time budget")
)

// deadlineCheckInterval controls how often the wall clock is sampled while budgeted units are counted.
const deadlineCheckInterval = 128

/*
budgetTracker holds the per-call state used to enforce a Budget.
A fresh tracker is created for every parse and every evaluation, so concurrent
calls never share budget state. All methods are safe to call on a nil receiver,
which represents the absence of any budget.
*/
type budgetTracker struct {
	maxTokens int
	maxDepth  int
	timeout   time.Duration
	deadline  time.Time

	tokens int
	depth  int
	ticks  int
}

func newBudgetTracker(budget Budget) *budgetTracker {

	if budget == (Budget{}) {
		return nil
	}

	ret := &budgetTracker{
		maxTokens: budget.MaxTokens,
		maxDepth:  budget.MaxDepth,
		timeout:   budget.Timeout,
	}
	if budget.Timeout > 0 {
		ret.deadline = time.Now().Add(budget.Timeout)
	}
	return ret
}

/*
addToken records one lexed token against the budget.
Token accounting happens during lexing, before any stage caching can take effect,
so cache hits cannot skip the budget.
*/
func (t *budgetTracker) addToken() error {

	if t == nil {
		return nil
	}

	t.tokens++
	if t.maxTokens > 0 && t.tokens > t.maxTokens {
		return fmt.Errorf("%w: %d tokens exceeds limit of %d", ErrTokenBudgetExceeded, t.tokens, t.maxTokens)
	}

	return t.tick()
}

/*
enter records descent into a nested expression level, either while planning stages
or while evaluating them.
*/
func (t *budgetTracker) enter() error {

	if t == nil {
		return nil
	}

	t.depth++
	if t.maxDepth > 0 && t.depth > t.maxDepth {
		t.depth--
		return fmt.Errorf("%w: nesting depth %d exceeds limit of %d", ErrDepthBudgetExceeded, t.depth+1, t.maxDepth)
	}

	return t.tick()
}

/*
exit records ascent from a nested expression level. It must be paired with a
successful call to enter.
*/
func (t *budgetTracker) exit() {

	if t == nil {
		return
	}

	t.depth--
}

/*
tick samples the wall clock every deadlineCheckInterval calls, so that the
common path only costs a counter increment.
*/
func (t *budgetTracker) tick() error {

	if t == nil || t.deadline.IsZero() {
		return nil
	}

	t.ticks++
	if t.ticks < deadlineCheckInterval {
		return nil
	}
	t.ticks = 0

	if time.Now().After(t.deadline) {
		return fmt.Errorf("%w: exceeded %v", ErrTimeoutBudgetExceeded, t.timeout)
	}

	return nil
}
