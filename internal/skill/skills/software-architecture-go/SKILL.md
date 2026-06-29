# Software Architecture in Go Skill

Deep architectural patterns for building CLI tools, structured logging systems, test frameworks, and observability pipelines in Go — distilled from production codebases (Cobra, Zap, Testify, OpenTelemetry, GitHub CLI).

## Overview

Software Architecture (Go) covers CLI patterns (Cobra), structured logging (Zap), testing patterns (Testify), observability (OpenTelemetry), and Go-specific architecture patterns like functional options, interface composition, and context propagation. Extracted from deep analysis of 5 production Go libraries.

**When to use**: Building, debugging, or optimizing systems in this domain.

## 1. CLI Architecture (Cobra)

### 1.1 Command Structure

Cobra's `Command` struct (from `cobra/command.go`) is the core abstraction — every CLI command is a tree of `Command` nodes:

```go
type Command struct {
    Use     string   // "command <arg> [flags]"
    Aliases []string // Alternative names
    Short   string   // One-line description for help
    Long    string   // Detailed description
    Example string   // Usage examples
    
    // Lifecycle hooks — executed in order:
    PersistentPreRun  func(cmd *Command, args []string)      // Inherited by children
    PreRun            func(cmd *Command, args []string)      // Current command only
    Run               func(cmd *Command, args []string)      // Main logic
    PostRun           func(cmd *Command, args []string)      // After Run
    PersistentPostRun func(cmd *Command, args []string)      // Inherited by children
    
    // Error-returning variants (preferred):
    PersistentPreRunE, PreRunE, RunE, PostRunE, PersistentPostRunE
    
    // Flag management
    flags  *flag.FlagSet  // All flags
    pflags *flag.FlagSet  // Persistent (inherited) flags
    
    // I/O abstraction
    inReader  io.Reader   // Replaces stdin
    outWriter io.Writer   // Replaces stdout
    errWriter io.Writer   // Replaces stderr
    
    // Command tree
    commands []*Command
    parent   *Command
    
    // Features
    TraverseChildren bool  // Parse flags on all parents before child
    Hidden           bool  // Hide from help
    SilenceErrors    bool  // Suppress error output
    SilenceUsage     bool  // Suppress usage on error
}
```

### 1.2 Options + Factory Pattern (GitHub CLI)

The canonical pattern for Go CLIs (from `cli/pkg/cmd/`):

```go
// 1. Options struct — all state for the command
type ListOptions struct {
    IO         *iostreams.IOStreams
    HttpClient func() (*http.Client, error)
    Config     func() (config.Config, error)
    BaseRepo   func() (ghrepo.Interface, error)
    
    // Flags
    Limit    int
    State    string
    Author   string
}

// 2. Constructor with test injection point
func NewCmdList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
    opts := &ListOptions{
        IO:         f.IOStreams,
        HttpClient: f.HttpClient,
        Config:     f.Config,
        BaseRepo:   f.BaseRepo,
    }
    
    cmd := &cobra.Command{
        Use:   "list",
        Short: "List items",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Lazy-init here, not in constructor
            repo, err := opts.BaseRepo()
            if err != nil {
                return err
            }
            
            if runF != nil {
                return runF(opts)
            }
            return listRun(opts)
        },
    }
    
    cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Max results")
    cmd.Flags().StringVarP(&opts.State, "state", "s", "open", "Filter by state")
    
    return cmd
}

// 3. Pure business logic — testable without Cobra
func listRun(opts *ListOptions) error {
    // ... implementation
}
```

**Hidden Gem — `runF` injection**: The `runF func(*ListOptions) error` parameter allows tests to inject custom logic without mocking the entire command:

```go
// Test pattern
func TestList(t *testing.T) {
    var gotOpts *ListOptions
    cmd := NewCmdList(factory, func(opts *ListOptions) error {
        gotOpts = opts
        return nil
    })
    cmd.SetArgs([]string{"--limit", "5"})
    cmd.Execute()
    assert.Equal(t, 5, gotOpts.Limit)
}
```

### 1.3 Global Configuration

Cobra provides global hooks and settings (from `cobra/cobra.go`):

```go
var EnablePrefixMatching = false    // Dangerous — disable by default
var EnableCommandSorting = true     // Alphabetical help ordering
var EnableCaseInsensitive = false   // Case-sensitive commands
var EnableTraverseRunHooks = false  // Only first run hook executes

// Global initializers — run before every command
func OnInitialize(y ...func()) {
    initializers = append(initializers, y...)
}

// Global finalizers — run after every command
func OnFinalize(y ...func()) {
    finalizers = append(finalizers, y...)
}
```

### 1.4 Command Groups and Help

```go
// Group commands in help output
cmd.AddGroup(&cobra.Group{ID: "core", Title: "Core Commands:"})
cmd.AddGroup(&cobra.Group{ID: "admin", Title: "Administration:"})

subcmd := &cobra.Command{GroupID: "core", ...}
```

### 1.5 Error Types (GitHub CLI)

```go
// From pkg/cmdutil/errors.go
FlagErrorf("invalid state: %s", state)     // Prints usage
&cmdutil.SilentError{}                       // Exit 1, no message
&cmdutil.CancelError{}                       // User cancelled
&cmdutil.PendingError{}                      // Outcome pending
&cmdutil.NoResultsError{}                    // Empty results

// Mutual exclusivity
cmdutil.MutuallyExclusive("cannot use --all with --specific", opts.All, opts.Specific)
```

---

## 2. Structured Logging (Zap)

### 2.1 Core Architecture

Zap's architecture is built on three layers:

```
Logger/SugaredLogger → zapcore.Core → Encoder + WriteSyncer
```

**The Core Interface** (from `zapcore/core.go`) — minimal, composable:

```go
type Core interface {
    LevelEnabler
    With([]Field) Core                    // Add structured context
    Check(Entry, *CheckedEntry) *CheckedEntry  // Should we log?
    Write(Entry, []Field) error           // Serialize and write
    Sync() error                          // Flush buffers
}
```

**The ioCore** — default implementation:

```go
type ioCore struct {
    LevelEnabler        // Level checking
    enc          Encoder     // JSON or console encoder
    out          WriteSyncer // Destination (stdout, file, etc.)
}
```

### 2.2 Logger vs SugaredLogger

```go
// Type-safe, zero-allocation Logger
logger.Info("message",
    zap.String("key", "value"),
    zap.Int("count", 42),
    zap.Duration("elapsed", time.Since(start)),
)

// Ergonomic SugaredLogger (slightly slower)
sugar.Infow("message",
    "key", "value",
    "count", 42,
    "elapsed", time.Since(start),
)
sugar.Infof("message: key=%s count=%d", "value", 42)

// Convert between them
sugar := logger.Sugar()
logger := sugar.Desugar()
```

**Hidden Gem — `Sugar()` adds callerSkip=2**: Because the SugaredLogger adds two extra stack frames, it automatically adjusts caller skip to report the correct source location.

### 2.3 Field System

Zap uses a **tagged union** pattern for zero-allocation fields (from `zapcore/field.go`):

```go
type Field struct {
    Key       string
    Type      FieldType    // Discriminator (28 types)
    Integer   int64        // Used for int*, uint*, float*, duration, time
    String    string       // Used for string type
    Interface interface{}  // Used for complex types (arrays, objects, errors)
}

// Factory functions — no heap allocation for primitives
func String(key, val string) Field    // → Field{Type: StringType, String: val}
func Int(key string, val int) Field   // → Field{Type: Int64Type, Integer: int64(val)}
func Bool(key string, val bool) Field // → Field{Type: BoolType, Integer: b2i(val)}
func Error(err error) Field           // → Field{Type: ErrorType, Interface: err}
func Duration(key string, val time.Duration) Field // → Field{Type: DurationType, Integer: int64(val)}
```

**Anti-Pattern**: Using `zap.Reflect()` for structured data — it uses `reflect.DeepEqual` and is 10-100x slower than typed fields.

### 2.4 Check-Write Pattern

The hot path in Zap uses a two-phase check-write pattern:

```go
func (log *Logger) Info(msg string, fields ...Field) {
    if ce := log.check(InfoLevel, msg); ce != nil {
        ce.Write(fields...)  // Only serialize if level is enabled
    }
}

func (log *Logger) check(lvl Level, msg string) *CheckedEntry {
    // Phase 1: Fast level check (no allocation)
    if lvl < DPanicLevel && !log.core.Enabled(lvl) {
        return nil  // Skip — zero cost for disabled levels
    }
    
    // Phase 2: Create checked entry with caller info
    ent := Entry{LoggerName: log.name, Time: log.clock.Now(), Level: lvl, Message: msg}
    ce := log.core.Check(ent, nil)
    
    // Add caller and stack trace if configured
    if log.addCaller { ... }
    if addStack { ... }
    
    return ce
}
```

**Hidden Gem**: For `Panic` and higher levels, Zap always creates a `CheckedEntry` even if the level is disabled, because terminal hooks (WriteThenPanic, WriteThenFatal) must execute regardless.

### 2.5 Logger Options

```go
// Functional options pattern
type Option interface {
    apply(*Logger)
}

logger, _ := zap.NewProduction(
    zap.AddCaller(),           // Add file:line
    zap.AddStacktrace(zap.ErrorLevel),  // Stack trace on errors
    zap.Fields(zap.String("service", "api")),  // Default fields
    zap.WrapCore(func(core zapcore.Core) zapcore.Core {
        return zapcore.NewTee(core, anotherCore)  // Multi-output
    }),
    zap.Hooks(func(entry zapcore.Entry) error {
        // Post-write hook (e.g., send to external system)
        return nil
    }),
    zap.WithLazy(),  // Lazy field evaluation
)
```

### 2.6 Sampling

Zap's sampler (from `zapcore/sampler.go`) reduces log volume by counting identical entries:

```go
// First N entries per time window are logged, then every Mth entry
type SamplerOption struct {
    Tick       time.Duration  // Sampling window (default: 1s)
    First      int            // Always log first N (default: 100)
    Thereafter int            // Log every Mth after (default: 100)
}
```

### 2.7 Testing Pattern

```go
// Observer pattern for testing
import "go.uber.org/zap/zaptest/observer"

observedZapCore, observedLogs := observer.New(zap.InfoLevel)
logger := zap.New(observedZapCore)

logger.Info("test message", zap.String("key", "value"))

logs := observedLogs.All()
assert.Equal(t, 1, len(logs))
assert.Equal(t, "test message", logs[0].Message)
assert.Equal(t, zap.InfoLevel, logs[0].Level)
```

---

## 3. Testing Patterns (Testify)

### 3.1 Assertion Architecture

Testify provides three assertion packages with distinct failure behaviors:

| Package | Behavior on Failure | Use Case |
|---|---|---|
| `assert` | Reports failure, continues test | Non-critical checks |
| `require` | Reports failure, stops test immediately | Critical preconditions |
| `mock` | Expectation-based mock framework | Dependency isolation |

**Rule**: Use `require` for error checks so the test halts immediately:

```go
require.NoError(t, err)    // Stops test on error
require.Error(t, err)      // Stops if no error
assert.Equal(t, expected, actual)  // Continues on mismatch
```

### 3.2 Key Assertion Types

```go
// Comparison assertions
assert.Equal(t, expected, actual)
assert.NotEqual(t, expected, actual)
assert.Contains(t, haystack, needle)
assert.ElementsMatch(t, listA, listB)  // Order-independent
assert.InDelta(t, 1.0, 0.99, 0.01)   // Float comparison

// Type assertions
assert.IsType(t, &MyStruct{}, actual)
assert.Implements(t, (*MyInterface)(nil), actual)

// Nil/empty assertions
assert.Nil(t, value)
assert.NotNil(t, value)
assert.Empty(t, collection)
assert.Len(t, collection, 5)

// Error assertions
assert.NoError(t, err)
assert.ErrorIs(t, err, ErrNotFound)
assert.ErrorContains(t, err, "not found")

// Boolean assertions
assert.True(t, condition)
assert.False(t, condition)
```

### 3.3 Custom Assertion Functions

Testify defines function types for table-driven tests:

```go
type ComparisonAssertionFunc = func(TestingT, interface{}, interface{}, ...interface{}) bool
type ValueAssertionFunc      = func(TestingT, interface{}, ...interface{}) bool
type ErrorAssertionFunc      = func(TestingT, error, ...interface{}) bool

// Table-driven test with assertion functions
tests := []struct {
    name      string
    input     interface{}
    assertion ValueAssertionFunc
}{
    {"valid", 42, assert.NotNil},
    {"invalid", nil, assert.Nil},
}
```

### 3.4 Mock Framework

From `mock/mock.go` — expectation-based mocking with fluent API:

```go
type Mock struct {
    ExpectedCalls []*Call
    mutex         sync.Mutex
}

type Call struct {
    Method          string
    Arguments       Arguments
    ReturnArguments Arguments
    Repeatability   int            // 0 = always, N = exactly N times
    WaitFor         <-chan time.Time  // Delayed response
    RunFn           func(Arguments)  // Side effects on call
    PanicMsg        *string          // Simulate panic
    requires        []*Call          // Ordering constraints
}

// Usage
mockService := new(MockService)
mockService.On("GetUser", 42).Return(&User{Name: "Alice"}, nil).Once()
mockService.On("GetUser", 99).Return(nil, ErrNotFound)

// Verification
defer mockService.AssertExpectations(t)
// or
mockService.AssertCalled(t, "GetUser", 42)
mockService.AssertNotCalled(t, "DeleteUser")
```

**Hidden Gem — `Run` callback**: Manipulate arguments passed by reference:

```go
mock.On("Unmarshal", mock.Anything).Run(func(args mock.Arguments) {
    arg := args.Get(0).(*MyStruct)
    arg.Field = "modified"
}).Return(nil)
```

### 3.5 Suite Pattern

```go
type DatabaseTestSuite struct {
    suite.Suite
    db     *sql.DB
    server *httptest.Server
}

func (s *DatabaseTestSuite) SetupSuite() {
    s.db = openTestDB()
    s.server = httptest.NewServer(handler)
}

func (s *DatabaseTestSuite) TearDownSuite() {
    s.db.Close()
    s.server.Close()
}

func (s *DatabaseTestSuite) SetupTest() {
    // Runs before each test
    s.db.Exec("TRUNCATE TABLE users")
}

func (s *DatabaseTestSuite) TestCreateUser() {
    user, err := s.db.CreateUser("alice")
    s.Require().NoError(err)
    s.Equal("alice", user.Name)
}

func TestDatabase(t *testing.T) {
    suite.Run(t, new(DatabaseTestSuite))
}
```

### 3.6 Caller Info Extraction

Testify's `CallerInfo()` walks the stack to find the actual test location, skipping internal frames:

```go
func CallerInfo() []string {
    // Walks stack, skips assert/mock/require packages
    // Stops at testing.tRunner
    // Returns file:line of the actual test code
}
```

---

## 4. Observability (OpenTelemetry)

### 4.1 TracerProvider Architecture

From `sdk/trace/provider.go` — the core trace pipeline:

```go
type TracerProvider struct {
    mu             sync.Mutex
    namedTracer    map[instrumentation.Scope]*tracer  // Cached tracers
    spanProcessors atomic.Pointer[spanProcessorStates] // Lock-free read
    isShutdown     atomic.Bool
    
    sampler     Sampler          // Sampling strategy
    idGenerator IDGenerator      // Span/Trace ID generation
    spanLimits  SpanLimits       // Attribute/event/link limits
    resource    *resource.Resource // Entity producing telemetry
}
```

### 4.2 Span Processor Pipeline

Spans flow through a chain of `SpanProcessor` implementations:

```go
type SpanProcessor interface {
    OnStart(parent context.Context, s ReadWriteSpan)     // Called when span starts
    OnEnd(s ReadOnlySpan)                                 // Called when span ends
    ForceFlush(ctx context.Context) error                 // Flush pending spans
    Shutdown(ctx context.Context) error                   // Cleanup
}
```

**Built-in Processors**:
- `SimpleSpanProcessor`: Synchronous export (testing only)
- `BatchSpanProcessor`: Buffered async export (production)

**Hidden Gem — Double-checked locking for shutdown**:

```go
func (p *TracerProvider) RegisterSpanProcessor(sp SpanProcessor) {
    if p.isShutdown.Load() { return }  // Fast path
    p.mu.Lock()
    defer p.mu.Unlock()
    if p.isShutdown.Load() { return }  // Re-check after lock
    // ... safe to register
}
```

### 4.3 Configuration Pattern

OpenTelemetry uses the functional options pattern with environment variable fallback:

```go
// Programmatic configuration
tp := trace.NewTracerProvider(
    trace.WithBatcher(exporter, trace.WithMaxExportBatchSize(512)),
    trace.WithResource(resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceName("my-service"),
        semconv.ServiceVersion("1.0.0"),
    )),
    trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(0.1))),
    trace.WithRawSpanLimits(trace.SpanLimits{
        AttributeCountLimit: 128,
        EventCountLimit:     128,
        LinkCountLimit:      128,
    }),
    trace.WithIDGenerator(customIDGenerator),
)

// Environment variable configuration (applied first, before programmatic opts)
// OTEL_TRACES_SAMPLER=parentbased_traceidratio
// OTEL_TRACES_SAMPLER_ARG=0.1
```

### 4.4 Sampling Strategies

```go
type Sampler interface {
    ShouldSample(parameters SamplingParameters) SamplingResult
}

// Built-in samplers
AlwaysSample()              // 100% — development
NeverSample()               // 0% — disable tracing
TraceIDRatioBased(0.01)     // 1% — production
ParentBased(delegate)       // Respect parent's decision

// ParentBased decision logic:
// - Root span (no parent): use delegate
// - Parent sampled: always sample
// - Parent not sampled: never sample
```

### 4.5 Span Limits

```go
type SpanLimits struct {
    AttributeCountLimit        int  // Max attributes per span
    AttributeValueLengthLimit  int  // Max string attribute length
    EventCountLimit            int  // Max events per span
    AttributePerEventCountLimit int // Max attributes per event
    LinkCountLimit             int  // Max links per span
    AttributePerLinkCountLimit int  // Max attributes per link
}

// Zero = disable that resource
// Negative = unlimited
// Default = use environment variables or sensible defaults
```

### 4.6 Lock-Free Span Processor States

```go
// atomic.Pointer for lock-free reads on the hot path
spanProcessors atomic.Pointer[spanProcessorStates]

// Read: lock-free (common path)
func (p *TracerProvider) getSpanProcessors() spanProcessorStates {
    return *p.spanProcessors.Load()
}

// Write: mutex-protected (rare path)
func (p *TracerProvider) RegisterSpanProcessor(sp SpanProcessor) {
    p.mu.Lock()
    defer p.mu.Unlock()
    current := p.getSpanProcessors()
    newSPS := make(spanProcessorStates, 0, len(current)+1)
    newSPS = append(newSPS, current...)
    newSPS = append(newSPS, newSpanProcessorState(sp))
    p.spanProcessors.Store(&newSPS)
}
```

---

## 5. Go Architecture Patterns

### 5.1 Functional Options Pattern

Used extensively across all libraries:

```go
// Define option type
type Option interface {
    apply(*Config)
}

type optionFunc func(*Config)
func (f optionFunc) apply(cfg *Config) { f(cfg) }

// Concrete options
func WithTimeout(d time.Duration) Option {
    return optionFunc(func(cfg *Config) { cfg.timeout = d })
}

// Usage
cfg := NewConfig(WithTimeout(5*time.Second), WithRetries(3))
```

### 5.2 Interface Segregation

Each library defines minimal interfaces:

```go
// Zap: 5 methods
type Core interface {
    LevelEnabler
    With([]Field) Core
    Check(Entry, *CheckedEntry) *CheckedEntry
    Write(Entry, []Field) error
    Sync() error
}

// Testify: 1 method
type TestingT interface {
    Errorf(format string, args ...interface{})
}

// OpenTelemetry: 4 methods
type SpanProcessor interface {
    OnStart(context.Context, ReadWriteSpan)
    OnEnd(ReadOnlySpan)
    ForceFlush(context.Context) error
    Shutdown(context.Context) error
}
```

### 5.3 Clone-on-Write Pattern

Both Zap and OpenTelemetry use immutable clone for thread safety:

```go
// Zap Logger — always clone before modifying
func (log *Logger) With(fields ...Field) *Logger {
    l := log.clone()          // Shallow copy
    l.core = l.core.With(fields)  // New core with fields
    return l
}

func (log *Logger) clone() *Logger {
    clone := *log  // Value copy
    return &clone
}

// OpenTelemetry TracerProvider — atomic pointer swap
p.spanProcessors.Store(&newSPS)  // Atomic replace
```

### 5.4 Error Handling Patterns

```go
// errors.Join for multiple errors (Go 1.20+)
var retErr error
for _, sps := range p.getSpanProcessors() {
    err := sps.sp.Shutdown(ctx)
    retErr = errors.Join(retErr, err)
}
return retErr

// Sentinel errors with Is()
var ErrNotFound = errors.New("not found")
if errors.Is(err, ErrNotFound) { ... }

// Custom error types
type FlagError struct { msg string }
func (e *FlagError) Error() string { return e.msg }
```

### 5.5 Testing Patterns

```go
// Table-driven tests
tests := []struct {
    name    string
    input   string
    want    int
    wantErr bool
}{
    {"empty", "", 0, true},
    {"valid", "hello", 5, false},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := Process(tt.input)
        if tt.wantErr {
            require.Error(t, err)
            return
        }
        require.NoError(t, err)
        assert.Equal(t, tt.want, got)
    })
}

// HTTP mocking (GitHub CLI pattern)
reg := &httpmock.Registry{}
defer reg.Verify(t)  // Ensure all stubs were called

reg.Register(
    httpmock.REST("GET", "repos/OWNER/REPO"),
    httpmock.JSONResponse(map[string]string{"name": "repo"}),
)
client := &http.Client{Transport: reg}

// IOStreams in tests
ios, stdin, stdout, stderr := iostreams.Test()
ios.SetStdoutTTY(true)  // Simulate terminal
```

### 5.6 Context Propagation

```go
// OpenTelemetry context propagation
ctx, span := tracer.Start(ctx, "operation",
    trace.WithAttributes(
        attribute.String("key", "value"),
        attribute.Int("count", 42),
    ),
    trace.WithSpanKind(trace.SpanKindServer),
)
defer span.End()

// Add events
span.AddEvent("processing", trace.WithAttributes(
    attribute.String("item.id", id),
))

// Record errors
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```

---

## 6. Implementation Internals

### 6.1 Zap Memory Optimization

**Buffer pool** for encoder output:
```go
// Reuse buffers to reduce GC pressure
bufferpool.Get()  // Get buffer from pool
buf.Free()        // Return to pool
```

**Stack trace capture** — lazy and cached:
```go
stack := stacktrace.Capture(log.callerSkip+callerSkipOffset, stackDepth)
defer stack.Free()
frame, more := stack.Next()
```

### 6.2 Cobra Levenshtein Suggestions

Cobra suggests commands based on edit distance when an unknown command is entered:

```go
func ld(s, t string, ignoreCase bool) int {
    // Dynamic programming Levenshtein distance
    // Used when DisableSuggestions = false
}
```

### 6.3 OpenTelemetry Attribute Deduplication

```go
// Deduplicate attributes on the hot path
attrs, _ := attrdedup.Set(c.InstrumentationAttributes())
```

### 6.4 Testify Reflect-Based Comparison

```go
func ObjectsAreEqual(expected, actual interface{}) bool {
    // Fast path for nil
    if expected == nil || actual == nil {
        return expected == actual
    }
    // Fast path for []byte
    exp, ok := expected.([]byte)
    if ok {
        act, ok := actual.([]byte)
        return ok && bytes.Equal(exp, act)
    }
    // Fallback to reflect.DeepEqual
    return reflect.DeepEqual(expected, actual)
}
```

**Hidden Gem — Exported fields only**: `ObjectsExportedFieldsAreEqual` recursively copies only exported fields before comparing, ignoring unexported state:

```go
func copyExportedFields(expected interface{}) interface{} {
    // Recursively strips unexported fields from structs
    // Handles nested structs, pointers, slices, maps
}
```

---

## 7. Anti-Patterns to Avoid

1. **Using `assert` instead of `require` for errors**: Tests continue after failed error checks, producing confusing cascading failures
2. **Synchronous span export in production**: `SimpleSpanProcessor` blocks the hot path; use `BatchSpanProcessor`
3. **Not calling `logger.Sync()` before exit**: Buffered logs are lost
4. **Creating new loggers per request**: Use `With()` to create child loggers; cloning is cheap
5. **Ignoring `atomic.Bool` for shutdown**: Using mutex-only shutdown checks leads to races
6. **Using `zap.Reflect()` on hot path**: 10-100x slower than typed fields
7. **Not using `defer reg.Verify(t)` in tests**: Mock expectations are silently unverified
8. **Hard-coding `github.com` as default host**: Use `cfg.Authentication().DefaultHost()` for GHES support
9. **Creating tracers outside the provider**: Always use `provider.Tracer()` for caching and correct instrumentation scope
10. **Not deduplicating attributes**: Duplicate attributes waste memory and can confuse backends


## Verification Checklist

- [ ] All commands have help text and examples
- [ ] Structured logging at appropriate levels
- [ ] Tests pass with `-race` flag
- [ ] Error messages are actionable (not just 'error occurred')
- [ ] Graceful shutdown implemented (context cancellation)
- [ ] Configuration validated at startup
- [ ] Metrics exposed for key operations
- [ ] README updated with usage examples
- [ ] Linting passes (golangci-lint)
- [ ] Cross-compilation tested (linux/darwin/windows)


---

## Decision tree

See **When to use** above.

## Workflow

1. Read the skill body above.
2. Identify the relevant section for your task.
3. Apply the patterns and examples provided.
4. Verify against the listed anti-patterns.

## Examples

Examples are interleaved throughout the skill body above.

## Anti-patterns

See the **When to use** criteria above. The skill is *not* applied when:
- The task is outside the skill's declared scope.
- Simpler alternatives exist.

## Failure modes

- **Misapplied scope**: invoking the skill for tasks it doesn't cover.
- **Outdated reference**: real-world library APIs may have shifted since synthesis.
- **Pattern drift**: the skill's patterns describe idealized APIs, not exact production code.

## Related skills

- `ai-agent-orchestration`
- `context-engineering`

