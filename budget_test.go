package govaluate

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBudget_TokenLimitExceeded(test *testing.T) {

	expression := strings.Repeat("1 + ", 100) + "1"

	_, err := NewEvaluableExpressionWithBudget(expression, Budget{MaxTokens: 50})
	if !errors.Is(err, ErrTokenBudgetExceeded) {
		test.Fatalf("expected ErrTokenBudgetExceeded, got: %v", err)
	}
}

func TestBudget_TokenLimitAllowed(test *testing.T) {

	expression := strings.Repeat("1 + ", 100) + "1"

	expr, err := NewEvaluableExpressionWithBudget(expression, Budget{MaxTokens: 1000})
	if err != nil {
		test.Fatalf("unexpected error: %v", err)
	}

	result, err := expr.Eval(nil)
	if err != nil {
		test.Fatalf("unexpected eval error: %v", err)
	}
	if result != 101.0 {
		test.Fatalf("expected 101, got: %v", result)
	}
}

func TestBudget_DepthLimitParentheses(test *testing.T) {

	expression := strings.Repeat("(", 200) + "1" + strings.Repeat(")", 200)

	_, err := NewEvaluableExpressionWithBudget(expression, Budget{MaxDepth: 50})
	if !errors.Is(err, ErrDepthBudgetExceeded) {
		test.Fatalf("expected ErrDepthBudgetExceeded, got: %v", err)
	}

	expr, err := NewEvaluableExpressionWithBudget(expression, Budget{MaxDepth: 300})
	if err != nil {
		test.Fatalf("unexpected error: %v", err)
	}

	result, err := expr.Eval(nil)
	if err != nil {
		test.Fatalf("unexpected eval error: %v", err)
	}
	if result != 1.0 {
		test.Fatalf("expected 1, got: %v", result)
	}
}

func TestBudget_DepthLimitBooleanChain(test *testing.T) {

	expression := strings.Repeat("true && ", 200) + "true"

	_, err := NewEvaluableExpressionWithBudget(expression, Budget{MaxDepth: 50})
	if !errors.Is(err, ErrDepthBudgetExceeded) {
		test.Fatalf("expected ErrDepthBudgetExceeded, got: %v", err)
	}
}

func TestBudget_DepthLimitPrefixChain(test *testing.T) {

	expression := strings.Repeat("!(", 200) + "true" + strings.Repeat(")", 200)

	_, err := NewEvaluableExpressionWithBudget(expression, Budget{MaxDepth: 50})
	if !errors.Is(err, ErrDepthBudgetExceeded) {
		test.Fatalf("expected ErrDepthBudgetExceeded, got: %v", err)
	}
}

func TestBudget_DepthLimitTernaryChain(test *testing.T) {

	expression := strings.Repeat("true ? 1 : ", 200) + "1"

	_, err := NewEvaluableExpressionWithBudget(expression, Budget{MaxDepth: 50})
	if !errors.Is(err, ErrDepthBudgetExceeded) {
		test.Fatalf("expected ErrDepthBudgetExceeded, got: %v", err)
	}
}

func TestBudget_DepthLimitFunctionNesting(test *testing.T) {

	functions := map[string]ExpressionFunction{
		"id": func(args ...interface{}) (interface{}, error) {
			return args[0], nil
		},
	}
	expression := strings.Repeat("id(", 200) + "1" + strings.Repeat(")", 200)

	_, err := NewEvaluableExpressionWithFunctionsAndBudget(expression, functions, Budget{MaxDepth: 50})
	if !errors.Is(err, ErrDepthBudgetExceeded) {
		test.Fatalf("expected ErrDepthBudgetExceeded, got: %v", err)
	}
}

func TestBudget_ParseTimeout(test *testing.T) {

	expression := strings.Repeat("1 + ", 500) + "1"

	_, err := NewEvaluableExpressionWithBudget(expression, Budget{Timeout: time.Nanosecond})
	if !errors.Is(err, ErrTimeoutBudgetExceeded) {
		test.Fatalf("expected ErrTimeoutBudgetExceeded, got: %v", err)
	}
}

func TestBudget_EvalDepthLimit(test *testing.T) {

	expression := strings.Repeat("a - ", 100) + "a"

	expr, err := NewEvaluableExpression(expression)
	if err != nil {
		test.Fatalf("unexpected error: %v", err)
	}
	expr.budget = Budget{MaxDepth: 10}

	_, err = expr.Eval(MapParameters(map[string]interface{}{"a": 1.0}))
	if !errors.Is(err, ErrDepthBudgetExceeded) {
		test.Fatalf("expected ErrDepthBudgetExceeded, got: %v", err)
	}
}

func TestBudget_EvalTimeout(test *testing.T) {

	expression := strings.Repeat("a + ", 500) + "a"

	expr, err := NewEvaluableExpression(expression)
	if err != nil {
		test.Fatalf("unexpected error: %v", err)
	}
	expr.budget = Budget{Timeout: time.Nanosecond}

	_, err = expr.Eval(MapParameters(map[string]interface{}{"a": 1.0}))
	if !errors.Is(err, ErrTimeoutBudgetExceeded) {
		test.Fatalf("expected ErrTimeoutBudgetExceeded, got: %v", err)
	}
}

func TestBudget_ShortCircuitPreserved(test *testing.T) {

	budget := Budget{MaxTokens: 100, MaxDepth: 100, Timeout: time.Minute}

	// "missing" is never resolved, because the left side short-circuits.
	expr, err := NewEvaluableExpressionWithBudget("false && missing > 0", budget)
	if err != nil {
		test.Fatalf("unexpected error: %v", err)
	}

	result, err := expr.Eval(nil)
	if err != nil {
		test.Fatalf("short-circuit broken: %v", err)
	}
	if result != false {
		test.Fatalf("expected false, got: %v", result)
	}

	expr, err = NewEvaluableExpressionWithBudget("true || missing > 0", budget)
	if err != nil {
		test.Fatalf("unexpected error: %v", err)
	}

	result, err = expr.Eval(nil)
	if err != nil {
		test.Fatalf("short-circuit broken: %v", err)
	}
	if result != true {
		test.Fatalf("expected true, got: %v", result)
	}

	expr, err = NewEvaluableExpressionWithBudget("1 ?? missing", budget)
	if err != nil {
		test.Fatalf("unexpected error: %v", err)
	}

	result, err = expr.Eval(nil)
	if err != nil {
		test.Fatalf("short-circuit broken: %v", err)
	}
	if result != 1.0 {
		test.Fatalf("expected 1, got: %v", result)
	}
}

func TestBudget_ValidExpressionUnchanged(test *testing.T) {

	budget := Budget{MaxTokens: 100, MaxDepth: 100, Timeout: time.Minute}

	expr, err := NewEvaluableExpressionWithBudget("(foo + 1) * 2 > 5 && bar == 'x'", budget)
	if err != nil {
		test.Fatalf("unexpected error: %v", err)
	}

	result, err := expr.Evaluate(map[string]interface{}{"foo": 2.0, "bar": "x"})
	if err != nil {
		test.Fatalf("unexpected eval error: %v", err)
	}
	if result != true {
		test.Fatalf("expected true, got: %v", result)
	}
}

func TestBudget_ZeroBudgetUnlimited(test *testing.T) {

	expression := strings.Repeat("(", 500) + "1" + strings.Repeat(")", 500)

	expr, err := NewEvaluableExpressionWithBudget(expression, Budget{})
	if err != nil {
		test.Fatalf("zero budget should not limit parsing: %v", err)
	}

	result, err := expr.Eval(nil)
	if err != nil {
		test.Fatalf("zero budget should not limit evaluation: %v", err)
	}
	if result != 1.0 {
		test.Fatalf("expected 1, got: %v", result)
	}
}

func TestBudget_ConcurrentCallsIsolated(test *testing.T) {

	expr, err := NewEvaluableExpressionWithBudget(
		"a * 2 + (b - 1) > 0 && !c",
		Budget{MaxTokens: 100, MaxDepth: 100, Timeout: time.Minute},
	)
	if err != nil {
		test.Fatalf("unexpected error: %v", err)
	}

	deepExpression := strings.Repeat("(", 500) + "1" + strings.Repeat(")", 500)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				result, err := expr.Eval(MapParameters(map[string]interface{}{
					"a": 2.0,
					"b": 3.0,
					"c": false,
				}))
				if err != nil {
					test.Errorf("worker %d: unexpected eval error: %v", worker, err)
					return
				}
				if result != true {
					test.Errorf("worker %d: expected true, got: %v", worker, result)
					return
				}

				// concurrently parsing an over-budget expression must not disturb
				// other goroutines' budgets.
				_, err = NewEvaluableExpressionWithBudget(deepExpression, Budget{MaxDepth: 10})
				if !errors.Is(err, ErrDepthBudgetExceeded) {
					test.Errorf("worker %d: expected ErrDepthBudgetExceeded, got: %v", worker, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}
