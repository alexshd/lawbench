package lawbench

import (
	"context"
	"testing"
	"time"
)

// Example verified type (would come from lawtest)
type VerifiedConfig struct {
	LawVerified // Embedded proof
	Data        map[string]string
}

// Example merge function
func MergeConfig(a, b VerifiedConfig) VerifiedConfig {
	result := VerifiedConfig{
		LawVerified: a.LawVerified,
		Data:        make(map[string]string),
	}
	for k, v := range a.Data {
		result.Data[k] = v
	}
	for k, v := range b.Data {
		result.Data[k] = v // Right wins
	}
	return result
}

// TestRuntimeLawChecker_Register verifies registration works.
func TestRuntimeLawChecker_Register(t *testing.T) {
	checker := NewRuntimeLawChecker()

	proof := LawVerified{
		TypeName:    "lawbench.VerifiedConfig",
		Laws:        []string{"Associative", "Commutative", "Idempotent"},
		TestedAt:    time.Now(),
		TestPackage: "lawbench_test",
		Properties: map[string]string{
			"merge": "right-wins",
		},
	}

	checker.Register(proof)

	// Verify retrieval
	retrieved, ok := checker.IsVerified("lawbench.VerifiedConfig")
	if !ok {
		t.Fatal("Type not found after registration")
	}

	if len(retrieved.Laws) != 3 {
		t.Errorf("Expected 3 laws, got %d", len(retrieved.Laws))
	}
}

// TestRuntimeLawChecker_CheckType_Verified verifies accepted types.
func TestRuntimeLawChecker_CheckType_Verified(t *testing.T) {
	checker := NewRuntimeLawChecker()

	proof := LawVerified{
		TypeName: "lawbench.VerifiedConfig",
		Laws:     []string{"Associative", "Commutative"},
		TestedAt: time.Now(),
	}
	checker.Register(proof)

	config := VerifiedConfig{
		LawVerified: proof,
		Data:        map[string]string{"key": "value"},
	}

	// Should accept: type is verified and has required laws
	err := checker.CheckType(config, []string{"Associative"})
	if err != nil {
		t.Errorf("CheckType rejected verified type: %v", err)
	}
}

// TestRuntimeLawChecker_CheckType_Unverified rejects unverified types.
func TestRuntimeLawChecker_CheckType_Unverified(t *testing.T) {
	checker := NewRuntimeLawChecker()

	// Unverified type (not in registry)
	type UnverifiedConfig struct {
		Data map[string]string
	}
	config := UnverifiedConfig{Data: map[string]string{"key": "value"}}

	// Should reject: type not in registry
	err := checker.CheckType(config, []string{"Associative"})
	if err == nil {
		t.Error("CheckType should reject unverified type")
	}

	t.Logf("✓ Correctly rejected: %v", err)
}

// TestRuntimeLawChecker_CheckType_MissingLaw rejects incomplete verification.
func TestRuntimeLawChecker_CheckType_MissingLaw(t *testing.T) {
	checker := NewRuntimeLawChecker()

	proof := LawVerified{
		TypeName: "lawbench.VerifiedConfig",
		Laws:     []string{"Associative"}, // Missing Commutative
		TestedAt: time.Now(),
	}
	checker.Register(proof)

	config := VerifiedConfig{
		LawVerified: proof,
		Data:        map[string]string{"key": "value"},
	}

	// Should reject: type verified but missing required law
	err := checker.CheckType(config, []string{"Associative", "Commutative"})
	if err == nil {
		t.Error("CheckType should reject type missing required law")
	}

	t.Logf("✓ Correctly rejected: %v", err)
}

// TestRuntimeLawChecker_SafeMerge_Success verifies safe merge.
func TestRuntimeLawChecker_SafeMerge_Success(t *testing.T) {
	checker := NewRuntimeLawChecker()

	proof := LawVerified{
		TypeName: "lawbench.VerifiedConfig",
		Laws:     []string{"Associative", "Commutative", "Idempotent"},
		TestedAt: time.Now(),
	}
	checker.Register(proof)

	a := VerifiedConfig{
		LawVerified: proof,
		Data:        map[string]string{"a": "1", "b": "2"},
	}
	b := VerifiedConfig{
		LawVerified: proof,
		Data:        map[string]string{"b": "3", "c": "4"},
	}

	ctx := context.Background()
	result, err := checker.SafeMerge(ctx, a, b, MergeConfig, []string{"Associative"})
	if err != nil {
		t.Fatalf("SafeMerge failed: %v", err)
	}

	merged := result.(VerifiedConfig)
	if merged.Data["a"] != "1" || merged.Data["b"] != "3" || merged.Data["c"] != "4" {
		t.Errorf("Merge result incorrect: %v", merged.Data)
	}

	t.Logf("✓ Merged: %v", merged.Data)
}

// TestRuntimeLawChecker_SafeMerge_RejectsUnverified rejects unverified types.
func TestRuntimeLawChecker_SafeMerge_RejectsUnverified(t *testing.T) {
	checker := NewRuntimeLawChecker()

	type Unverified struct {
		Data map[string]string
	}
	a := Unverified{Data: map[string]string{"a": "1"}}
	b := Unverified{Data: map[string]string{"b": "2"}}

	mergeFn := func(x, y Unverified) Unverified {
		return Unverified{Data: map[string]string{}}
	}

	ctx := context.Background()
	_, err := checker.SafeMerge(ctx, a, b, mergeFn, []string{"Associative"})
	if err == nil {
		t.Error("SafeMerge should reject unverified types")
	}

	t.Logf("✓ Correctly rejected: %v", err)
}

// TestRuntimeLawChecker_ValidateBoundary demonstrates API boundary check.
func TestRuntimeLawChecker_ValidateBoundary(t *testing.T) {
	checker := NewRuntimeLawChecker()

	proof := LawVerified{
		TypeName: "lawbench.VerifiedConfig",
		Laws:     []string{"Associative", "Commutative"},
		TestedAt: time.Now(),
	}
	checker.Register(proof)

	// Simulate receiving data from external API
	externalData := VerifiedConfig{
		LawVerified: proof,
		Data:        map[string]string{"external": "data"},
	}

	// Validate at boundary
	err := checker.ValidateBoundary(externalData, []string{"Associative"})
	if err != nil {
		t.Errorf("ValidateBoundary rejected valid external data: %v", err)
	}

	t.Log("✓ External data validated and accepted")
}

// TestRuntimeLawChecker_EmbeddedDetection verifies embedded LawVerified extraction.
func TestRuntimeLawChecker_EmbeddedDetection(t *testing.T) {
	checker := NewRuntimeLawChecker()

	// Don't register - rely on embedded detection
	proof := LawVerified{
		TypeName: "lawbench.VerifiedConfig",
		Laws:     []string{"Associative"},
		TestedAt: time.Now(),
	}

	config := VerifiedConfig{
		LawVerified: proof,
		Data:        map[string]string{"key": "value"},
	}

	// Should extract embedded LawVerified
	err := checker.CheckType(config, []string{"Associative"})
	if err != nil {
		t.Errorf("CheckType failed to extract embedded LawVerified: %v", err)
	}

	t.Log("✓ Embedded LawVerified detected and validated")
}

// ExampleRuntimeLawChecker demonstrates real-world usage.
func ExampleRuntimeLawChecker() {
	// Setup: Register verified types (done once at startup)
	checker := NewRuntimeLawChecker()
	checker.Register(LawVerified{
		TypeName: "myapp.Config",
		Laws:     []string{"Associative", "Commutative", "Idempotent"},
		TestedAt: time.Now(),
	})

	// Runtime: Validate external data at API boundary
	// externalConfig := receiveFromAPI()
	// err := checker.ValidateBoundary(externalConfig, []string{"Associative"})
	// if err != nil {
	//     return fmt.Errorf("rejecting untrusted config: %w", err)
	// }
	// Safe to use now!
}
