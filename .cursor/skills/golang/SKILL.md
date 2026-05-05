---
name: golang
description: Domain skill for Go services and tooling. Covers project structure, error handling, concurrency, HTTP servers, structured logging (slog), testing, observability, dependency injection, generics, and module management. Use when working on any Go code. Applies to **/*.go, **/go.mod, **/go.sum files.
---

# Go Development

## Domain
Go services, CLIs, libraries, and internal tooling.

## Toolchain
- **Language**: Go 1.23+
- **Modules**: `go mod`
- **Testing**: `go test` with `-race`, `-cover`, `-shuffle`
- **Linting**: `golangci-lint` (aggregates staticcheck, errcheck, govet, gosec, etc.)
- **Formatting**: `gofmt` / `goimports`
- **Logging**: `log/slog` (stdlib structured logging)
- **Build**: `go build` with `-ldflags` for version injection
- **Profiling**: `pprof`, `trace`

## Quality Gate Commands

```bash
goimports -w .
go vet ./...
golangci-lint run ./...
go test ./... -race -cover -shuffle=on -count=1
go build ./...
```

Run linters before tests — lint failures are cheaper to fix than test failures.

## Project Structure

```
service/
├── cmd/
│   └── server/
│       └── main.go           # Entry point — wiring only, no logic
├── internal/                  # Compile-time import protection
│   ├── handler/               # HTTP/gRPC handlers (transport layer)
│   ├── service/               # Business logic (domain layer)
│   ├── repository/            # Data access (persistence layer)
│   ├── model/                 # Domain types and value objects
│   ├── middleware/            # HTTP middleware (auth, logging, recovery)
│   └── config/                # Configuration loading and validation
├── pkg/                       # Public packages (only if reusable across repos)
├── api/                       # OpenAPI specs, protobuf definitions
├── migrations/                # Database migrations
├── .golangci.yml              # Linter configuration
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

Rules:
- `cmd/` — thin entry points that wire dependencies and start the server
- `internal/` — all application code; the Go compiler prevents external imports
- `pkg/` — only create if code is genuinely reusable across projects; don't use as a dump
- Organize `internal/` by layer (handler/service/repository) or by domain — pick one per project and stay consistent
- Each directory is a package — don't create directories just for organization

> Ref: https://go.dev/doc/modules/layout

## Critical Patterns

### 1. Error Handling

> Ref: https://go.dev/blog/error-handling-and-go
> Ref: https://go.dev/doc/effective_go#errors

```go
// BAD — discarding errors
data, _ := json.Marshal(payload)

// BAD — generic error message, no wrapping
if err != nil {
    return errors.New("something went wrong")
}

// BAD — logging and returning (double-reporting)
if err != nil {
    log.Printf("failed: %v", err)
    return err
}
```

```go
// GOOD — wrap with context using %w
result, err := repo.GetByID(ctx, userID)
if err != nil {
    return nil, fmt.Errorf("getting user %s: %w", userID, err)
}

// GOOD — sentinel errors for control flow
var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

if errors.Is(err, ErrNotFound) {
    return nil, status.Error(codes.NotFound, "user not found")
}

// GOOD — custom error types for rich context
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

var ve *ValidationError
if errors.As(err, &ve) {
    // handle validation-specific logic
}
```

Rules:
- Handle every error explicitly — never use `_` to discard unless documented
- Wrap errors with business context at abstraction boundaries (`%w` verb)
- Use `errors.Is()` for sentinel errors, `errors.As()` for typed errors
- Either log or return an error — never both (causes double-reporting)
- Define sentinel errors (`var Err...`) at the package level for expected failure cases
- Return errors from the deepest point with context; callers add their own layer

### 2. Context Propagation

> Ref: https://pkg.go.dev/context
> Ref: https://go.dev/blog/context

```go
// BAD — storing context in a struct
type Server struct {
    ctx context.Context
}

// BAD — using context.Background() deep in the call stack
func (r *Repo) Get(id string) (*User, error) {
    return r.db.QueryContext(context.Background(), query, id)
}

// BAD — context not the first parameter
func Process(req *Request, ctx context.Context) error { ... }
```

```go
// GOOD — context as first parameter, always
func (s *Service) Process(ctx context.Context, req *Request) (*Response, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return s.repo.Get(ctx, req.ID)
}

// GOOD — propagate context through the entire call chain
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    user, err := h.service.GetUser(r.Context(), r.PathValue("id"))
    // ...
}
```

Rules:
- Context is always the first parameter, named `ctx`
- Never store context in a struct
- Use `context.WithTimeout` / `context.WithCancel` for deadline propagation
- Always call `defer cancel()` after creating a derived context
- Use `context.Background()` only in `main()`, tests, and top-level init
- Add values to context sparingly — only for request-scoped cross-cutting data (request ID, trace ID)

### 3. Interfaces

> Ref: https://go.dev/doc/effective_go#interfaces
> Ref: https://go.dev/wiki/CodeReviewComments#interfaces

```go
// BAD — large interface defined by the implementor
type UserService interface {
    Create(ctx context.Context, u *User) error
    GetByID(ctx context.Context, id string) (*User, error)
    Update(ctx context.Context, u *User) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter Filter) ([]*User, error)
    Count(ctx context.Context) (int, error)
    Export(ctx context.Context, format string) ([]byte, error)
}

// BAD — interface defined in the same package as implementation
package user

type Repository interface { ... }
type repository struct { ... }
```

```go
// GOOD — small interface defined by the consumer
package handler

type UserGetter interface {
    GetByID(ctx context.Context, id string) (*User, error)
}

type Handler struct {
    users UserGetter
}

// GOOD — the bigger the interface, the weaker the abstraction
type Reader interface {
    Read(p []byte) (n int, err error)
}
```

Rules:
- **Accept interfaces, return structs** — the Go proverb
- Define interfaces where they are used (consumer package), not where they are implemented
- Keep interfaces small — 1-3 methods is ideal
- Don't create interfaces preemptively; extract when you need a second implementation or a test mock
- The standard library's `io.Reader`, `io.Writer`, `fmt.Stringer` are the gold standard

### 4. Concurrency

> Ref: https://go.dev/doc/effective_go#concurrency
> Ref: https://go.dev/blog/pipelines

```go
// BAD — fire-and-forget goroutine (leak risk)
go processEvent(event)

// BAD — goroutine without cancellation mechanism
go func() {
    for {
        doWork()
        time.Sleep(time.Second)
    }
}()

// BAD — shared state protected by both mutex and channel (pick one)
type Counter struct {
    mu sync.Mutex
    ch chan int
    n  int
}
```

```go
// GOOD — errgroup for parallel work with error propagation and cancellation
g, ctx := errgroup.WithContext(ctx)
for _, item := range items {
    g.Go(func() error {
        return process(ctx, item)
    })
}
if err := g.Wait(); err != nil {
    return fmt.Errorf("processing items: %w", err)
}

// GOOD — bounded worker pool
func processAll(ctx context.Context, items []Item, workers int) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(workers)
    for _, item := range items {
        g.Go(func() error {
            return process(ctx, item)
        })
    }
    return g.Wait()
}

// GOOD — graceful goroutine with context cancellation
func (w *Worker) Run(ctx context.Context) error {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := w.doWork(ctx); err != nil {
                return fmt.Errorf("worker tick: %w", err)
            }
        }
    }
}
```

Rules:
- Never launch goroutines without a cancellation mechanism (`context`, `done` channel)
- Use `errgroup` for parallel work — it handles synchronization, cancellation, and error collection
- Use `errgroup.SetLimit(n)` for bounded concurrency (worker pool)
- Protect shared state with `sync.Mutex` OR channels — never both
- Use `sync.Once` for one-time initialization, not `sync.Mutex` with a bool flag
- Use `sync.Map` only for cache-like access patterns with many goroutines; prefer `map` + `sync.RWMutex` otherwise
- Always run tests with `-race` flag

### 5. Functional Options

> Ref: https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis

```go
// BAD — config struct with many booleans
server := NewServer("localhost", 8080, true, false, 30, nil, true)

// BAD — too many constructor parameters
func NewServer(host string, port int, tls bool, ...) *Server
```

```go
// GOOD — functional options pattern
type Option func(*Server)

func WithTLS(certFile, keyFile string) Option {
    return func(s *Server) {
        s.certFile = certFile
        s.keyFile = keyFile
    }
}

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{addr: addr, timeout: 30 * time.Second}
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
srv := NewServer(":8080",
    WithTLS("cert.pem", "key.pem"),
    WithTimeout(60 * time.Second),
)
```

Use functional options when a constructor has more than 3 parameters or optional configuration.

### 6. Configuration

```go
// BAD — reading env vars scattered throughout the code
func (s *Service) Process() {
    timeout, _ := strconv.Atoi(os.Getenv("TIMEOUT"))
    // ...
}
```

```go
// GOOD — centralized config struct, validated at startup
type Config struct {
    Port        int           `env:"PORT" envDefault:"8080"`
    DatabaseURL string        `env:"DATABASE_URL,required"`
    Timeout     time.Duration `env:"TIMEOUT" envDefault:"30s"`
    LogLevel    slog.Level    `env:"LOG_LEVEL" envDefault:"info"`
}

func LoadConfig() (*Config, error) {
    var cfg Config
    if err := env.Parse(&cfg); err != nil {
        return nil, fmt.Errorf("parsing config: %w", err)
    }
    return &cfg, nil
}

// In main.go — fail fast on bad config
func main() {
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("config: %v", err)
    }
    // ...
}
```

Rules:
- Load and validate all configuration at startup in `main()` — fail fast
- Use a single config struct, not scattered `os.Getenv` calls
- Use `envDefault` tags for sensible defaults
- Required fields should cause startup failure if missing
- Never log config values that contain secrets

## HTTP Server Patterns

> Ref: https://pkg.go.dev/net/http (Go 1.22+ enhanced routing)

### Router Setup (Go 1.22+ stdlib)

```go
mux := http.NewServeMux()

mux.HandleFunc("GET /health", h.Health)
mux.HandleFunc("GET /api/users/{id}", h.GetUser)
mux.HandleFunc("POST /api/users", h.CreateUser)
mux.HandleFunc("PUT /api/users/{id}", h.UpdateUser)
mux.HandleFunc("DELETE /api/users/{id}", h.DeleteUser)
```

Go 1.22+ supports method-based routing and path parameters (`{id}`) in the standard library. Use `r.PathValue("id")` to extract path parameters.

### Middleware

```go
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            next.ServeHTTP(w, r)
            logger.Info("request",
                "method", r.Method,
                "path", r.URL.Path,
                "duration", time.Since(start),
            )
        })
    }
}

func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                slog.Error("panic recovered", "error", err, "stack", string(debug.Stack()))
                http.Error(w, "internal server error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

### Graceful Shutdown

```go
func main() {
    srv := &http.Server{
        Addr:         cfg.Addr,
        Handler:      handler,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    errCh := make(chan error, 1)
    go func() { errCh <- srv.ListenAndServe() }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    select {
    case sig := <-quit:
        slog.Info("shutting down", "signal", sig)
    case err := <-errCh:
        slog.Error("server error", "error", err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        slog.Error("forced shutdown", "error", err)
    }
}
```

Rules:
- Always set `ReadTimeout`, `WriteTimeout`, `IdleTimeout` on `http.Server`
- Handle `SIGINT`/`SIGTERM` for graceful shutdown
- Use `srv.Shutdown(ctx)` — it drains in-flight requests before stopping
- Prefer Go 1.22+ stdlib routing over third-party routers for new projects
- Use `chi` or `gin` only when you need complex middleware ecosystems or legacy compat

### Health Endpoint

```go
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
    if err := h.db.PingContext(r.Context()); err != nil {
        http.Error(w, "not ready", http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

- `/health` (liveness) — always returns 200 if the process is alive
- `/ready` (readiness) — checks dependencies (DB, cache) before returning 200

## Structured Logging with slog

> Ref: https://go.dev/blog/slog
> Ref: https://pkg.go.dev/log/slog

```go
// BAD — unstructured printf-style logging
log.Printf("user %s created in %v", userID, duration)

// BAD — third-party logger when slog suffices
logger := logrus.New()
logger.WithField("user_id", userID).Info("user created")
```

```go
// GOOD — slog with JSON handler (production)
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: cfg.LogLevel,
}))
slog.SetDefault(logger)

slog.Info("user created",
    "user_id", userID,
    "duration_ms", duration.Milliseconds(),
)

// GOOD — logger with persistent attributes
logger = logger.With("service", "user-api", "version", version)

// GOOD — context-enriched logging
func (s *Service) Process(ctx context.Context, id string) error {
    logger := slog.With("request_id", middleware.RequestID(ctx))
    logger.Info("processing", "id", id)
    // ...
}

// GOOD — slog groups for structured sub-objects
slog.Info("request completed",
    slog.Group("http",
        slog.String("method", r.Method),
        slog.String("path", r.URL.Path),
        slog.Int("status", status),
    ),
    slog.Duration("duration", elapsed),
)
```

Rules:
- Use `log/slog` (stdlib) — no third-party loggers needed for new projects
- Use `slog.NewJSONHandler` in production, `slog.NewTextHandler` for local dev
- Use `slog.With()` to create loggers with persistent attributes (service name, version)
- Log levels: `Debug` (dev only), `Info` (business events), `Warn` (unexpected but handled), `Error` (failures requiring attention)
- Never log: passwords, tokens, PII, full request bodies with sensitive data
- Include correlation/request ID in every log line

## Testing Patterns

> Ref: https://go.dev/doc/tutorial/add-a-test
> Ref: https://pkg.go.dev/testing

### Table-Driven Tests

```go
func TestParseAmount(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name    string
        input   string
        want    int64
        wantErr bool
    }{
        {name: "valid amount", input: "42.50", want: 4250},
        {name: "zero", input: "0.00", want: 0},
        {name: "negative", input: "-10", want: -1000},
        {name: "invalid", input: "abc", wantErr: true},
        {name: "empty", input: "", wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got, err := ParseAmount(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Mocking with Interfaces

```go
type mockRepo struct {
    getUserFn func(ctx context.Context, id string) (*User, error)
}

func (m *mockRepo) GetByID(ctx context.Context, id string) (*User, error) {
    return m.getUserFn(ctx, id)
}

func TestServiceGetUser(t *testing.T) {
    repo := &mockRepo{
        getUserFn: func(_ context.Context, id string) (*User, error) {
            if id == "123" {
                return &User{ID: "123", Name: "Alice"}, nil
            }
            return nil, ErrNotFound
        },
    }
    svc := NewService(repo)
    user, err := svc.GetUser(context.Background(), "123")
    require.NoError(t, err)
    assert.Equal(t, "Alice", user.Name)
}
```

### HTTP Handler Tests

```go
func TestGetUserHandler(t *testing.T) {
    h := NewHandler(&mockService{...})

    req := httptest.NewRequest("GET", "/api/users/123", nil)
    req.SetPathValue("id", "123")
    rec := httptest.NewRecorder()

    h.GetUser(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)

    var user User
    require.NoError(t, json.NewDecoder(rec.Body).Decode(&user))
    assert.Equal(t, "123", user.ID)
}
```

### Benchmarks

```go
func BenchmarkParseAmount(b *testing.B) {
    for b.Loop() {
        ParseAmount("42.50")
    }
}
```

Run with `go test -bench=. -benchmem ./...`

### Build Tags for Integration Tests

```go
//go:build integration

package repository_test

func TestPostgresRepository(t *testing.T) {
    db := setupTestDB(t)
    // ...
}
```

Run with `go test -tags=integration ./...`

Rules:
- Use table-driven tests for multiple input/output scenarios
- `t.Parallel()` on test function and subtests for independent tests
- `require` for preconditions (stops test), `assert` for checks (continues test)
- `httptest.NewRecorder` + `httptest.NewRequest` for handler tests
- Build tags to separate unit and integration tests
- `-shuffle=on` to catch order-dependent tests
- `-race` to detect data races
- `-count=1` to disable test caching during development

## Dependency Injection

```go
// BAD — hard-coded dependencies
type Handler struct{}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    db, _ := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    user, _ := db.QueryRow("SELECT ...").Scan(...)
}
```

```go
// GOOD — constructor injection
type Handler struct {
    service UserService
    logger  *slog.Logger
}

func NewHandler(svc UserService, logger *slog.Logger) *Handler {
    return &Handler{service: svc, logger: logger}
}

// main.go — wire everything at the top
func main() {
    cfg := config.MustLoad()
    db := database.MustConnect(cfg.DatabaseURL)
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    repo := repository.NewPostgresRepo(db)
    svc := service.NewUserService(repo)
    handler := handler.NewHandler(svc, logger)

    mux := http.NewServeMux()
    mux.HandleFunc("GET /api/users/{id}", handler.GetUser)
    // ...
}
```

Rules:
- Wire dependencies in `main()` — it's the composition root
- Pass dependencies via constructors, not global state or `init()`
- Use `Must*` prefixed constructors for startup-critical dependencies that should panic on failure
- Avoid DI frameworks (`wire`, `fx`) unless the project already uses one — manual wiring is explicit and debuggable
- Interfaces for testing, structs for wiring

## Go Modules & Build

> Ref: https://go.dev/ref/mod

```bash
go mod init github.com/org/service   # Initialize module
go mod tidy                           # Clean up go.mod/go.sum
go mod download                       # Download dependencies
go get package@v1.2.3                 # Add/update dependency
go get package@latest                 # Update to latest
go mod verify                         # Verify integrity
```

### Makefile

```makefile
.PHONY: build test lint fmt run

VERSION ?= $(shell git describe --tags --always --dirty)

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/server ./cmd/server

test:
	go test ./... -race -cover -shuffle=on -count=1

lint:
	golangci-lint run ./...

fmt:
	goimports -w .

run: build
	./bin/server
```

### Multi-Stage Docker Build

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /server /server
USER nonroot:nonroot
ENTRYPOINT ["/server"]
```

Rules:
- Multi-stage build: Go builder + distroless runtime
- `CGO_ENABLED=0` for static binaries (no libc dependency)
- `-ldflags="-s -w"` to strip debug info (smaller binary)
- Copy `go.mod`/`go.sum` first for Docker layer caching
- Run as non-root user
- Never vendor unless the project already does

## Linter Configuration

```yaml
# .golangci.yml
run:
  timeout: 5m
  go: "1.23"

linters:
  enable:
    - errcheck        # unchecked errors
    - govet           # suspicious constructs
    - staticcheck     # advanced static analysis
    - unused          # unused code
    - gosec           # security issues
    - gocritic        # opinionated checks
    - errorlint       # error wrapping issues
    - bodyclose       # unclosed HTTP response bodies
    - noctx           # http requests without context
    - prealloc        # slice preallocation
    - exhaustive      # missing enum cases in switch
    - revive          # extensible linter (replaces golint)

linters-settings:
  gocritic:
    enabled-tags:
      - diagnostic
      - performance
  revive:
    rules:
      - name: unexported-return
        disabled: true

issues:
  exclude-rules:
    - path: _test\.go
      linters: [gosec, errcheck]
```

Essential linters explained:
- `errcheck` — catches discarded errors (the #1 Go bug source)
- `errorlint` — ensures proper `%w` wrapping and `errors.Is`/`errors.As` usage
- `bodyclose` — catches unclosed `http.Response.Body` (resource leak)
- `noctx` — flags HTTP requests without `context.Context`
- `gosec` — detects security issues (hardcoded credentials, SQL injection)
- `exhaustive` — ensures switch statements cover all enum values

## Modern Go Features

### Generics (Go 1.18+)

```go
// GOOD — generic utility functions
func Map[T, U any](items []T, fn func(T) U) []U {
    result := make([]U, len(items))
    for i, item := range items {
        result[i] = fn(item)
    }
    return result
}

func Filter[T any](items []T, fn func(T) bool) []T {
    var result []T
    for _, item := range items {
        if fn(item) {
            result = append(result, item)
        }
    }
    return result
}

// GOOD — generic with constraints
func Min[T cmp.Ordered](a, b T) T {
    if a < b {
        return a
    }
    return b
}
```

Use generics for:
- Collection utilities (`Map`, `Filter`, `Reduce`)
- Type-safe containers (sets, ordered maps)
- Algorithms that work across numeric types

Avoid generics for:
- Interfaces that work fine without them
- When it makes the code harder to read than type-specific functions

### Enhanced HTTP Routing (Go 1.22+)

```go
mux.HandleFunc("GET /api/users/{id}", handler.GetUser)
mux.HandleFunc("POST /api/users", handler.CreateUser)

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    // ...
}
```

The stdlib router now supports HTTP method matching and path parameters — no third-party router required for most APIs.

### Range Over Function (Go 1.23+)

```go
// Iterator pattern using range-over-func
func (db *DB) AllUsers(ctx context.Context) iter.Seq2[*User, error] {
    return func(yield func(*User, error) bool) {
        rows, err := db.QueryContext(ctx, "SELECT ...")
        if err != nil {
            yield(nil, err)
            return
        }
        defer rows.Close()
        for rows.Next() {
            var u User
            if err := rows.Scan(&u.ID, &u.Name); err != nil {
                if !yield(nil, err) { return }
                continue
            }
            if !yield(&u, nil) { return }
        }
    }
}

// Usage
for user, err := range db.AllUsers(ctx) {
    if err != nil { return err }
    process(user)
}
```

Use `iter.Seq` / `iter.Seq2` for lazy iteration over large datasets, database rows, or paginated API results.

## Observability

### Prometheus Metrics

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path", "status"},
    )
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )
)

func init() {
    prometheus.MustRegister(requestDuration, requestsTotal)
}
```

### OpenTelemetry Tracing

```go
import "go.opentelemetry.io/otel"

tracer := otel.Tracer("user-service")

func (s *Service) GetUser(ctx context.Context, id string) (*User, error) {
    ctx, span := tracer.Start(ctx, "Service.GetUser")
    defer span.End()

    span.SetAttributes(attribute.String("user.id", id))

    user, err := s.repo.GetByID(ctx, id)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }
    return user, nil
}
```

Rules:
- Expose `/metrics` endpoint for Prometheus scraping
- Use histograms for latency, counters for requests/errors
- Keep label cardinality low (no user IDs or request IDs as labels)
- Propagate trace context through `context.Context`
- Record errors on spans with `span.RecordError(err)`

## Anti-Pattern Quick Reference

| Anti-pattern | Why it's bad | Fix |
|-------------|-------------|-----|
| `err` discarded with `_` | Silent failures | Handle every error explicitly |
| Log and return error | Double-reporting in logs | Either log or return, not both |
| `errors.New` without context | Undebuggable error chains | Use `fmt.Errorf("context: %w", err)` |
| Context stored in struct | Context is request-scoped, not object-scoped | Pass `ctx` as first function parameter |
| `context.Background()` in library code | Ignores caller's cancellation/timeout | Accept `ctx` from caller |
| Fire-and-forget goroutine | Goroutine leak, unhandled panics | Use `errgroup` or explicit lifecycle |
| `sync.Mutex` + channel on same state | Deadlock risk, hard to reason about | Pick one synchronization primitive |
| `init()` for complex setup | Implicit, untestable, order-dependent | Explicit initialization in `main()` |
| Global mutable state | Race conditions, untestable | Constructor injection |
| Printf-style logging | Unstructured, hard to parse/filter | Use `log/slog` with key-value pairs |
| `:latest` base image in Docker | Non-reproducible builds | Pin Go version (`golang:1.23-alpine`) |
| `interface{}` / `any` everywhere | Loses type safety | Use generics or concrete types |
| Large interfaces | Tight coupling, hard to mock | 1-3 method interfaces at consumer site |
| Missing HTTP server timeouts | Slowloris attacks, resource exhaustion | Set Read/Write/Idle timeouts |
| No `-race` in tests | Undetected data races | Always test with `-race` flag |

## Official Documentation Links

| Topic | URL |
|-------|-----|
| Effective Go | https://go.dev/doc/effective_go |
| Go Code Review Comments | https://go.dev/wiki/CodeReviewComments |
| Google Go Style Guide | https://google.github.io/styleguide/go/ |
| Go Module Reference | https://go.dev/ref/mod |
| Go Module Layout | https://go.dev/doc/modules/layout |
| Standard Library | https://pkg.go.dev/std |
| Go Blog: Error Handling | https://go.dev/blog/error-handling-and-go |
| Go Blog: Context | https://go.dev/blog/context |
| Go Blog: Structured Logging (slog) | https://go.dev/blog/slog |
| Go Blog: Range Over Functions | https://go.dev/blog/range-functions |
| Go Blog: Pipelines & Cancellation | https://go.dev/blog/pipelines |
| Go Common Mistakes | https://go.dev/wiki/CommonMistakes |
| Go 1.23 Release Notes | https://go.dev/doc/go1.23 |
| Go Security Best Practices | https://go.dev/doc/security/best-practices |
| golangci-lint | https://golangci-lint.run/ |
| testify | https://pkg.go.dev/github.com/stretchr/testify |

## Related Skills
- `docker-containers` — distroless images, multi-stage builds for Go binaries
- `auto-qa` — Go test patterns, coverage analysis, mutation testing
- `cicd-github-actions` — Go build, test, lint in CI pipelines
- `monitoring` — Prometheus metrics, OpenTelemetry tracing, alerting
- `security` — input validation, dependency auditing, gosec findings
- `database-operations` — connection pooling, migrations, query patterns
