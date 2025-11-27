package lawbench

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// LawVerified marks a type as having passed lawtest verification at compile time.
// Types embedding this can be trusted in production.
type LawVerified struct {
	TypeName    string            // Fully qualified type name
	Laws        []string          // Which laws passed: "Associative", "Commutative", etc.
	TestedAt    time.Time         // When tests passed
	TestPackage string            // Where tests live
	Properties  map[string]string // Additional metadata
}

// RuntimeLawChecker validates unknown types at runtime using reflection.
type RuntimeLawChecker struct {
	// Registry of verified types (populated at test time)
	verified map[string]LawVerified
}

// NewRuntimeLawChecker creates a checker with an empty registry.
func NewRuntimeLawChecker() *RuntimeLawChecker {
	return &RuntimeLawChecker{
		verified: make(map[string]LawVerified),
	}
}

// Register adds a verified type to the runtime registry.
// Call this during init() or test setup after lawtest passes.
func (r *RuntimeLawChecker) Register(v LawVerified) {
	r.verified[v.TypeName] = v
}

// IsVerified checks if a type has passed lawtest at compile time.
func (r *RuntimeLawChecker) IsVerified(typeName string) (LawVerified, bool) {
	v, ok := r.verified[typeName]
	return v, ok
}

// CheckType validates an unknown value received from outside.
// Returns error if type is not verified or doesn't implement required laws.
func (r *RuntimeLawChecker) CheckType(v interface{}, requiredLaws []string) error {
	t := reflect.TypeOf(v)
	if t == nil {
		return fmt.Errorf("nil value cannot be verified")
	}

	typeName := t.String()

	// Check if type is in registry
	verified, ok := r.verified[typeName]
	if !ok {
		// Type not verified - check if it embeds LawVerified
		if embed := r.extractEmbedded(v); embed != nil {
			verified = *embed
			ok = true
		}
	}

	if !ok {
		return fmt.Errorf("type %s not in verified registry (did it pass lawtest?)", typeName)
	}

	// Check if it implements required laws
	for _, required := range requiredLaws {
		if !contains(verified.Laws, required) {
			return fmt.Errorf("type %s missing required law: %s (has: %v)",
				typeName, required, verified.Laws)
		}
	}

	return nil
}

// extractEmbedded checks if value embeds LawVerified struct.
func (r *RuntimeLawChecker) extractEmbedded(v interface{}) *LawVerified {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	// Look for embedded LawVerified field
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		if field.Type == reflect.TypeOf(LawVerified{}) {
			lv := val.Field(i).Interface().(LawVerified)
			return &lv
		}
	}

	return nil
}

// SafeMerge attempts to merge two values using a merge function.
// Validates both values are verified before merging.
// Returns error if types are incompatible or unverified.
//
// PERFORMANCE WARNING: This uses reflection (slow). Suitable for CONTROL PLANE only.
// For DATA PLANE (event folding, hot path), use code generation or Go generics.
// Reflection overhead: ~1000ns per call vs ~1ns for direct call.
// Violates Power conservation (T × S × P) if used in tight loops.
func (r *RuntimeLawChecker) SafeMerge(
	ctx context.Context,
	a, b interface{},
	mergeFn interface{}, // func(A, A) A
	requiredLaws []string,
) (interface{}, error) {
	// Validate inputs
	if err := r.CheckType(a, requiredLaws); err != nil {
		return nil, fmt.Errorf("first argument: %w", err)
	}
	if err := r.CheckType(b, requiredLaws); err != nil {
		return nil, fmt.Errorf("second argument: %w", err)
	}

	// Validate types match
	ta := reflect.TypeOf(a)
	tb := reflect.TypeOf(b)
	if ta != tb {
		return nil, fmt.Errorf("type mismatch: %s != %s", ta, tb)
	}

	// Validate merge function signature
	fnVal := reflect.ValueOf(mergeFn)
	fnType := fnVal.Type()
	if fnType.Kind() != reflect.Func {
		return nil, fmt.Errorf("mergeFn must be a function, got %s", fnType.Kind())
	}
	if fnType.NumIn() != 2 || fnType.NumOut() != 1 {
		return nil, fmt.Errorf("mergeFn must have signature func(T, T) T, got %s", fnType)
	}
	if fnType.In(0) != ta || fnType.In(1) != ta || fnType.Out(0) != ta {
		return nil, fmt.Errorf("mergeFn signature mismatch: expected func(%s, %s) %s", ta, ta, ta)
	}

	// Execute merge
	args := []reflect.Value{reflect.ValueOf(a), reflect.ValueOf(b)}
	results := fnVal.Call(args)
	return results[0].Interface(), nil
}

// MustMerge is like SafeMerge but panics on error.
// Use when you've already validated types at system boundary.
func (r *RuntimeLawChecker) MustMerge(
	ctx context.Context,
	a, b interface{},
	mergeFn interface{},
	requiredLaws []string,
) interface{} {
	result, err := r.SafeMerge(ctx, a, b, mergeFn, requiredLaws)
	if err != nil {
		panic(fmt.Sprintf("lawbench: merge failed: %v", err))
	}
	return result
}

// ValidateBoundary checks untrusted input at system boundary.
// This is the key insight: use reflection to test compatibility at runtime!
//
// Example:
//
//	// At API boundary
//	func HandleExternalConfig(raw interface{}) error {
//	    checker := GetGlobalChecker() // singleton
//	    if err := checker.ValidateBoundary(raw, []string{"Associative", "Commutative"}); err != nil {
//	        return fmt.Errorf("rejecting untrusted config: %w", err)
//	    }
//	    // Now safe to use
//	    return processConfig(raw)
//	}
func (r *RuntimeLawChecker) ValidateBoundary(v interface{}, requiredLaws []string) error {
	return r.CheckType(v, requiredLaws)
}

// contains checks if slice contains string.
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// Global singleton (optional convenience)
var globalChecker = NewRuntimeLawChecker()

// Register adds to global registry.
func Register(v LawVerified) {
	globalChecker.Register(v)
}

// CheckType validates against global registry.
func CheckType(v interface{}, requiredLaws []string) error {
	return globalChecker.CheckType(v, requiredLaws)
}

// ValidateBoundary validates against global registry.
func ValidateBoundary(v interface{}, requiredLaws []string) error {
	return globalChecker.ValidateBoundary(v, requiredLaws)
}
