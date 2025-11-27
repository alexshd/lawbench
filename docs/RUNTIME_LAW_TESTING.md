# Runtime Law Testing with Reflection

## Concept

**lawtest** validates algebraic properties at **compile time** (test time).  
**Runtime law checking** validates types at **system boundaries** using **reflection**.

This allows you to:

1. Test types with lawtest during development ✅
2. Register verified types in a runtime registry ✅
3. Reject untrusted external data that hasn't passed lawtest ✅

## Philosophy

> "If a type hasn't passed lawtest, it's untrusted and should be rejected at the API boundary."

Traditional approach:

```
External API → Accept anything → Hope it's correct → Runtime bugs
```

lawbench approach:

```
External API → Check if type passed lawtest → Reject if not → Type-safe system
```

## Usage

### 1. Test Your Type (Compile Time)

```go
// In your_type_test.go
func TestConfig_Associative(t *testing.T) {
    gen := func() Config {
        return Config{Data: map[string]string{
            lawtest.StringGen(5)(): lawtest.StringGen(10)(),
        }}
    }
    lawtest.Associative(t, MergeConfig, gen)
}

func TestConfig_Commutative(t *testing.T) {
    lawtest.Commutative(t, MergeConfig, gen)
}

func TestConfig_Idempotent(t *testing.T) {
    lawtest.Idempotent(t, MergeConfig, gen)
}
```

### 2. Register Verified Type (Runtime)

```go
// In init() or main()
func init() {
    lawbench.Register(lawbench.LawVerified{
        TypeName: "myapp.Config",
        Laws:     []string{"Associative", "Commutative", "Idempotent"},
        TestedAt: time.Now(),
        TestPackage: "myapp_test",
    })
}
```

### 3. Validate at API Boundary (Runtime)

```go
// At API boundary
func HandleExternalConfig(raw interface{}) error {
    // Validate type passed lawtest
    err := lawbench.ValidateBoundary(raw, []string{"Associative"})
    if err != nil {
        return fmt.Errorf("rejecting untrusted config: %w", err)
    }

    // Now safe to use!
    config := raw.(Config)
    return processConfig(config)
}
```

## Real-World Example

### Scenario: CRDT Merge from Untrusted Source

```go
// Your CRDT type (tested with lawtest)
type MyCRDT struct {
    lawbench.LawVerified  // Embed proof
    Data map[string]int
}

func MergeCRDT(a, b MyCRDT) MyCRDT {
    result := MyCRDT{Data: make(map[string]int)}
    for k, v := range a.Data {
        result.Data[k] = max(v, b.Data[k])
    }
    for k, v := range b.Data {
        if _, ok := result.Data[k]; !ok {
            result.Data[k] = v
        }
    }
    return result
}

// During testing: lawtest validates MergeCRDT is associative, commutative, idempotent
// At runtime: Validate external CRDTs before merging

func ReceiveCRDTFromNetwork(conn net.Conn) (MyCRDT, error) {
    var external interface{}
    dec := json.NewDecoder(conn)
    if err := dec.Decode(&external); err != nil {
        return MyCRDT{}, err
    }

    // CRITICAL: Validate before using
    checker := lawbench.NewRuntimeLawChecker()
    err := checker.CheckType(external, []string{"Associative", "Commutative", "Idempotent"})
    if err != nil {
        return MyCRDT{}, fmt.Errorf("untrusted CRDT rejected: %w", err)
    }

    return external.(MyCRDT), nil
}
```

## API

### Register

```go
lawbench.Register(lawbench.LawVerified{
    TypeName:    "fully.qualified.TypeName",
    Laws:        []string{"Associative", "Commutative", "Idempotent"},
    TestedAt:    time.Now(),
    TestPackage: "mypackage_test",
    Properties: map[string]string{
        "merge-strategy": "last-write-wins",
    },
})
```

### CheckType

```go
err := lawbench.CheckType(value, []string{"Associative"})
if err != nil {
    // Type not verified or missing required law
}
```

### ValidateBoundary

```go
err := lawbench.ValidateBoundary(externalValue, []string{"Associative", "Commutative"})
if err != nil {
    return fmt.Errorf("rejecting: %w", err)
}
```

### SafeMerge

```go
result, err := checker.SafeMerge(ctx, a, b, MergeFunc, []string{"Associative"})
if err != nil {
    // Type mismatch or unverified
}
```

## Embedding LawVerified

Types can embed `LawVerified` to carry their proof:

```go
type MyType struct {
    lawbench.LawVerified  // Proof of correctness
    Data map[string]string
}

func NewMyType() MyType {
    return MyType{
        LawVerified: lawbench.LawVerified{
            TypeName: "mypackage.MyType",
            Laws:     []string{"Associative", "Commutative"},
            TestedAt: time.Now(),
        },
        Data: make(map[string]string),
    }
}
```

The runtime checker will **automatically extract** embedded proofs.

## Use Cases

### 1. Distributed Config Merge

- Node A and Node B independently update config
- Merge at runtime: validate both configs passed lawtest
- Guarantees: Associative + Commutative = order doesn't matter

### 2. CRDT Replication

- Receive CRDT from untrusted network peer
- Validate: Associative + Commutative + Idempotent
- Safe to merge without coordinator

### 3. Plugin System

- External plugins provide merge functions
- Validate: Functions operate on law-tested types
- Reject plugins that use untested types

### 4. API Gateway

- External services send data for aggregation
- Validate: Data types passed lawtest before aggregating
- Prevents corruption from untested external types

## Testing the Runtime Checker

```bash
cd backend
go test ./lawbench -v -run TestRuntimeLawChecker
```

All tests pass ✅:

- Register/retrieve verified types
- Accept verified types with required laws
- Reject unverified types
- Reject types missing required laws
- Safe merge with validation
- Embedded LawVerified detection

## Design Decisions

### Why Reflection?

Go's `reflect` package allows runtime type inspection:

```go
t := reflect.TypeOf(value)
typeName := t.String()  // "mypackage.MyType"
```

This enables checking the registry without compile-time knowledge.

### Why Registry?

The registry maps `TypeName → LawVerified`:

```go
map[string]LawVerified{
    "myapp.Config": {Laws: []string{"Associative", ...}},
}
```

Populated during `init()` or test setup after lawtest passes.

### Why NOT a Type Parameter?

Go generics can't express "type that passed lawtest" as a constraint.  
Reflection is the only way to check arbitrary types at runtime.

## Connection to lawtest

**lawtest** (compile time):

- Generates random inputs
- Verifies algebraic properties
- Fails if laws don't hold
- Blocks deployment

**lawbench runtime checking** (runtime):

- Checks if type passed lawtest
- Rejects untrusted external data
- Validates at system boundaries
- Prevents runtime corruption

Together: **Mathematical correctness from development to production**.

## Future Enhancements

1. **Automatic Registration**: Generate init() from lawtest results
2. **Serialization**: Include LawVerified in JSON/Cap'n Proto
3. **Network Protocol**: Send proof with data over the wire
4. **Monitoring**: Track rejected types (security/debugging)
5. **Fuzzing Integration**: Fuzz runtime checker with random types

## Status

✅ Implemented: `lawbench/runtime.go` (193 lines)  
✅ Tested: `lawbench/runtime_test.go` (235 lines)  
✅ All tests passing

---

**This is potentially novel**: Runtime algebraic property validation using reflection.  
No other system (that I know of) validates lawtest properties at API boundaries.
