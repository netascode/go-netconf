# Concurrency Guide

This guide explains thread-safety, concurrent operations, and best practices for using go-netconf in concurrent environments.

## Table of Contents

- [Thread-Safety Model](#thread-safety-model)
- [Concurrent Read Operations](#concurrent-read-operations)
- [Synchronized Write Operations](#synchronized-write-operations)
- [Multiple Client Instances](#multiple-client-instances)
- [Patterns and Examples](#patterns-and-examples)
- [Best Practices](#best-practices)
- [Common Pitfalls](#common-pitfalls)

## Thread-Safety Model

go-netconf implements a **readers-writer lock** pattern for thread safety:

**Read Operations (Concurrent-Safe):**
- `Get()` - Multiple goroutines can call simultaneously
- `GetConfig()` - Multiple goroutines can call simultaneously
- `Validate()` - Multiple goroutines can call simultaneously (read-only validation)
- `ServerCapabilities()` - Thread-safe with mutex protection
- `ServerHasCapability()` - Thread-safe with mutex protection

**Write Operations (Serialized):**
- `EditConfig()` - Mutex-protected, only one at a time
- `CopyConfig()` - Mutex-protected, only one at a time
- `DeleteConfig()` - Mutex-protected, only one at a time
- `Lock()` - Mutex-protected, only one at a time
- `Unlock()` - Mutex-protected, only one at a time
- `Commit()` - Mutex-protected, only one at a time
- `Discard()` - Mutex-protected, only one at a time
- `RPC()` - Mutex-protected (may modify state)

**Internal Synchronization:**
```go
type Client struct {
    mu sync.RWMutex  // Read-write mutex for synchronization
    // ...
}
```

**Lock Ordering:**
- Read operations acquire `RLock()` (shared lock)
- Write operations acquire `Lock()` (exclusive lock)
- Multiple readers can execute concurrently
- Writers are serialized and block readers

## Concurrent Read Operations

Multiple goroutines can safely execute read operations simultaneously:

### Example 1: Concurrent Get Operations

```go
func main() {
    client, err := netconf.NewClient("192.168.1.1",
        netconf.Username("admin"),
        netconf.Password("secret"))
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Launch concurrent Get operations
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            filter := netconf.SubtreeFilter("<interfaces/>")
            res, err := client.Get(ctx, filter)
            if err != nil {
                log.Printf("Goroutine %d failed: %v", id, err)
                return
            }

            log.Printf("Goroutine %d succeeded: %d bytes", id, len(res.Res.String()))
        }(i)
    }

    wg.Wait()
    log.Println("All read operations completed")
}
```

### Example 2: Concurrent GetConfig with Different Filters

```go
func fetchMultipleSections(ctx context.Context, client *netconf.Client) error {
    type section struct {
        name   string
        filter netconf.Filter
    }

    sections := []section{
        {"system", netconf.SubtreeFilter("<system/>")},
        {"interfaces", netconf.SubtreeFilter("<interfaces/>")},
        {"routing", netconf.SubtreeFilter("<routing/>")},
    }

    // Concurrent fetch
    results := make(chan error, len(sections))
    for _, s := range sections {
        go func(sec section) {
            res, err := client.GetConfig(ctx, "running", sec.filter)
            if err != nil {
                results <- fmt.Errorf("%s failed: %w", sec.name, err)
                return
            }

            log.Printf("%s: %d bytes", sec.name, len(res.Res.String()))
            results <- nil
        }(s)
    }

    // Wait for all results
    for range sections {
        if err := <-results; err != nil {
            return err
        }
    }

    return nil
}
```

### Example 3: Concurrent Reads with Rate Limiting

```go
func fetchWithRateLimit(ctx context.Context, client *netconf.Client,
                        filters []netconf.Filter, rateLimit int) error {
    // Create semaphore for rate limiting
    sem := make(chan struct{}, rateLimit)

    var wg sync.WaitGroup
    errChan := make(chan error, len(filters))

    for i, filter := range filters {
        wg.Add(1)
        go func(id int, f netconf.Filter) {
            defer wg.Done()

            // Acquire semaphore
            sem <- struct{}{}
            defer func() { <-sem }()

            res, err := client.Get(ctx, f)
            if err != nil {
                errChan <- fmt.Errorf("query %d failed: %w", id, err)
                return
            }

            log.Printf("Query %d: %d bytes", id, len(res.Res.String()))
        }(i, filter)
    }

    wg.Wait()
    close(errChan)

    // Check for errors
    for err := range errChan {
        if err != nil {
            return err
        }
    }

    return nil
}
```

## Synchronized Write Operations

Write operations are automatically serialized by the library's mutex. However, you must still coordinate multiple writers at the application level:

### Example 1: Sequential Writes

```go
func applyConfigurations(ctx context.Context, client *netconf.Client, configs []string) error {
    // Lock candidate once for all operations
    if _, err := client.Lock(ctx, "candidate"); err != nil {
        return err
    }
    // CRITICAL: Always use defer with error handling
    defer func() {
        if _, err := client.Unlock(ctx, "candidate"); err != nil {
            log.Printf("Warning: Failed to unlock: %v", err)
        }
    }()

    // Sequential edits (library ensures serialization)
    for i, config := range configs {
        _, err := client.EditConfig(ctx, "candidate", config)
        if err != nil {
            client.Discard(ctx)
            return fmt.Errorf("config %d failed: %w", i, err)
        }
        log.Printf("Applied config %d", i)
    }

    // Validate and commit
    if _, err := client.Validate(ctx, "candidate"); err != nil {
        client.Discard(ctx)
        return err
    }

    _, err := client.Commit(ctx)
    return err
}
```

### Example 2: Coordinated Writes with Channel

```go
type ConfigRequest struct {
    Config string
    Result chan error
}

func configWorker(ctx context.Context, client *netconf.Client, requests <-chan ConfigRequest) {
    for req := range requests {
        _, err := client.EditConfig(ctx, "candidate", req.Config)
        req.Result <- err
    }
}

func main() {
    client, _ := netconf.NewClient("192.168.1.1",
        netconf.Username("admin"),
        netconf.Password("secret"))
    defer client.Close()

    ctx := context.Background()

    // Single worker to serialize writes
    requests := make(chan ConfigRequest, 10)
    go configWorker(ctx, client, requests)

    // Submit requests from multiple goroutines
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            config := fmt.Sprintf("<system><hostname>router%d</hostname></system>", id)
            result := make(chan error, 1)

            requests <- ConfigRequest{Config: config, Result: result}

            if err := <-result; err != nil {
                log.Printf("Config %d failed: %v", id, err)
            } else {
                log.Printf("Config %d succeeded", id)
            }
        }(i)
    }

    wg.Wait()
    close(requests)
}
```

### Example 3: Write Queue with Batching

```go
type ConfigQueue struct {
    client  *netconf.Client
    queue   chan string
    mu      sync.Mutex
    batch   []string
    maxSize int
}

func NewConfigQueue(client *netconf.Client, maxSize int) *ConfigQueue {
    q := &ConfigQueue{
        client:  client,
        queue:   make(chan string, 100),
        maxSize: maxSize,
    }
    go q.worker()
    return q
}

func (q *ConfigQueue) Add(config string) {
    q.queue <- config
}

func (q *ConfigQueue) worker() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case config := <-q.queue:
            q.mu.Lock()
            q.batch = append(q.batch, config)
            if len(q.batch) >= q.maxSize {
                q.flush()
            }
            q.mu.Unlock()

        case <-ticker.C:
            q.mu.Lock()
            if len(q.batch) > 0 {
                q.flush()
            }
            q.mu.Unlock()
        }
    }
}

func (q *ConfigQueue) flush() {
    if len(q.batch) == 0 {
        return
    }

    ctx := context.Background()

    // Lock, edit, commit
    if _, err := q.client.Lock(ctx, "candidate"); err != nil {
        log.Printf("Lock failed: %v", err)
        return
    }
    // CRITICAL: Always use defer with error handling
    defer func() {
        if _, err := q.client.Unlock(ctx, "candidate"); err != nil {
            log.Printf("Warning: Failed to unlock: %v", err)
        }
    }()

    for _, config := range q.batch {
        if _, err := q.client.EditConfig(ctx, "candidate", config); err != nil {
            log.Printf("Edit failed: %v", err)
            q.client.Discard(ctx)
            return
        }
    }

    if _, err := q.client.Commit(ctx); err != nil {
        log.Printf("Commit failed: %v", err)
    } else {
        log.Printf("Flushed %d configs", len(q.batch))
    }

    q.batch = nil
}
```

## Multiple Client Instances

Each Client instance is thread-safe, but you may want multiple clients for different purposes:

### Example 1: Read and Write Clients

```go
type NetconfManager struct {
    readClient  *netconf.Client
    writeClient *netconf.Client
}

func NewNetconfManager(host, username, password string) (*NetconfManager, error) {
    // Client optimized for reads
    readClient, err := netconf.NewClient(host,
        netconf.Username(username),
        netconf.Password(password),
        netconf.OperationTimeout(30*time.Second))
    if err != nil {
        return nil, err
    }

    // Client optimized for writes
    writeClient, err := netconf.NewClient(host,
        netconf.Username(username),
        netconf.Password(password),
        netconf.OperationTimeout(5*time.Minute),
        netconf.MaxRetries(5))
    if err != nil {
        readClient.Close()
        return nil, err
    }

    return &NetconfManager{
        readClient:  readClient,
        writeClient: writeClient,
    }, nil
}

func (m *NetconfManager) Close() error {
    err1 := m.readClient.Close()
    err2 := m.writeClient.Close()
    if err1 != nil {
        return err1
    }
    return err2
}

func (m *NetconfManager) GetConfig(ctx context.Context, source string,
                                   filter netconf.Filter) (netconf.Res, error) {
    return m.readClient.GetConfig(ctx, source, filter)
}

func (m *NetconfManager) EditConfig(ctx context.Context, target, config string) (netconf.Res, error) {
    return m.writeClient.EditConfig(ctx, target, config)
}
```

### Example 2: Client Pool

```go
import (
    "fmt"
    "log"
    "sync/atomic"

    "github.com/netascode/go-netconf"
)

type ClientPool struct {
    clients []*netconf.Client
    next    uint64
}

func NewClientPool(host, username, password string, size int) (*ClientPool, error) {
    pool := &ClientPool{
        clients: make([]*netconf.Client, 0, size),
    }

    for i := 0; i < size; i++ {
        client, err := netconf.NewClient(host,
            netconf.Username(username),
            netconf.Password(password))
        if err != nil {
            // Close existing clients on error
            pool.Close()
            return nil, err
        }
        pool.clients = append(pool.clients, client)
    }

    return pool, nil
}

func (p *ClientPool) Get() *netconf.Client {
    // Round-robin selection with atomic counter
    // atomic.AddUint64 is thread-safe
    idx := atomic.AddUint64(&p.next, 1) % uint64(len(p.clients))
    return p.clients[idx]
}

func (p *ClientPool) Close() error {
    var errs []error
    for _, client := range p.clients {
        if err := client.Close(); err != nil {
            errs = append(errs, err)
        }
    }
    if len(errs) > 0 {
        return fmt.Errorf("pool close errors: %v", errs)
    }
    return nil
}

// Usage
func main() {
    pool, _ := NewClientPool("192.168.1.1", "admin", "secret", 5)
    defer pool.Close()

    // Concurrent operations with different clients
    var wg sync.WaitGroup
    for i := 0; i < 20; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            client := pool.Get()
            ctx := context.Background()
            filter := netconf.SubtreeFilter("<interfaces/>")
            _, err := client.GetConfig(ctx, "running", filter)
            if err != nil {
                log.Printf("Request %d failed: %v", id, err)
            }
        }(i)
    }
    wg.Wait()
}
```

## Patterns and Examples

### Pattern 1: Fan-Out Read, Sequential Write

```go
func fetchAndApply(ctx context.Context, client *netconf.Client) error {
    // Fan-out: Concurrent reads
    type configData struct {
        name string
        data netconf.Res
        err  error
    }

    sections := []string{"system", "interfaces", "routing"}
    results := make(chan configData, len(sections))

    for _, section := range sections {
        go func(name string) {
            filter := netconf.SubtreeFilter(fmt.Sprintf("<%s/>", name))
            res, err := client.GetConfig(ctx, "running", filter)
            results <- configData{name: name, data: res, err: err}
        }(section)
    }

    // Collect results
    configs := make(map[string]netconf.Res)
    for range sections {
        result := <-results
        if result.err != nil {
            return result.err
        }
        configs[result.name] = result.data
    }

    // Sequential write
    if _, err := client.Lock(ctx, "candidate"); err != nil {
        return err
    }
    defer func() {
        if _, err := client.Unlock(ctx, "candidate"); err != nil {
            log.Printf("Warning: Failed to unlock: %v", err)
        }
    }()

    for name, config := range configs {
        modified := processConfig(config)  // Application-specific
        _, err := client.EditConfig(ctx, "candidate", modified)
        if err != nil {
            client.Discard(ctx)
            return fmt.Errorf("%s edit failed: %w", name, err)
        }
    }

    _, err := client.Commit(ctx)
    return err
}
```

### Pattern 2: Pipeline Processing

```go
type Pipeline struct {
    client  *netconf.Client
    readers int
    writers int
}

func NewPipeline(client *netconf.Client, readers, writers int) *Pipeline {
    return &Pipeline{
        client:  client,
        readers: readers,
        writers: writers,
    }
}

func (p *Pipeline) Process(ctx context.Context, filters []netconf.Filter) error {
    // Stage 1: Read (concurrent)
    readResults := make(chan netconf.Res, len(filters))
    readErrors := make(chan error, len(filters))

    sem := make(chan struct{}, p.readers)
    var wg sync.WaitGroup

    for _, filter := range filters {
        wg.Add(1)
        go func(f netconf.Filter) {
            defer wg.Done()

            sem <- struct{}{}
            defer func() { <-sem }()

            res, err := p.client.Get(ctx, f)
            if err != nil {
                readErrors <- err
                return
            }
            readResults <- res
        }(filter)
    }

    go func() {
        wg.Wait()
        close(readResults)
        close(readErrors)
    }()

    // Check read errors
    select {
    case err := <-readErrors:
        if err != nil {
            return err
        }
    default:
    }

    // Stage 2: Transform (concurrent)
    transformResults := make(chan string, len(filters))

    for res := range readResults {
        wg.Add(1)
        go func(r netconf.Res) {
            defer wg.Done()

            config := transformConfig(r)  // Application-specific
            transformResults <- config
        }(res)
    }

    go func() {
        wg.Wait()
        close(transformResults)
    }()

    // Stage 3: Write (serialized)
    if _, err := p.client.Lock(ctx, "candidate"); err != nil {
        return err
    }
    defer func() {
        if _, err := p.client.Unlock(ctx, "candidate"); err != nil {
            log.Printf("Warning: Failed to unlock: %v", err)
        }
    }()

    for config := range transformResults {
        _, err := p.client.EditConfig(ctx, "candidate", config)
        if err != nil {
            p.client.Discard(ctx)
            return err
        }
    }

    _, err := p.client.Commit(ctx)
    return err
}

func transformConfig(res netconf.Res) string {
    // Application-specific transformation
    return res.Res.String()
}
```

### Pattern 3: Concurrent Monitoring

```go
type Monitor struct {
    client   *netconf.Client
    interval time.Duration
    filters  []netconf.Filter
    results  chan MonitorResult
}

type MonitorResult struct {
    Timestamp time.Time
    Filter    netconf.Filter
    Data      netconf.Res
    Error     error
}

func NewMonitor(client *netconf.Client, interval time.Duration,
                filters []netconf.Filter) *Monitor {
    return &Monitor{
        client:   client,
        interval: interval,
        filters:  filters,
        results:  make(chan MonitorResult, 100),
    }
}

func (m *Monitor) Start(ctx context.Context) {
    ticker := time.NewTicker(m.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            close(m.results)
            return

        case <-ticker.C:
            // Concurrent fetch
            for _, filter := range m.filters {
                go func(f netconf.Filter) {
                    res, err := m.client.Get(ctx, f)
                    m.results <- MonitorResult{
                        Timestamp: time.Now(),
                        Filter:    f,
                        Data:      res,
                        Error:     err,
                    }
                }(filter)
            }
        }
    }
}

func (m *Monitor) Results() <-chan MonitorResult {
    return m.results
}

// Usage
func main() {
    client, _ := netconf.NewClient("192.168.1.1",
        netconf.Username("admin"),
        netconf.Password("secret"))
    defer client.Close()

    filters := []netconf.Filter{
        netconf.SubtreeFilter("<interfaces/>"),
        netconf.SubtreeFilter("<system/>"),
    }

    monitor := NewMonitor(client, 10*time.Second, filters)

    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
    defer cancel()

    go monitor.Start(ctx)

    for result := range monitor.Results() {
        if result.Error != nil {
            log.Printf("Monitor error: %v", result.Error)
            continue
        }
        log.Printf("Monitor result at %v: %d bytes",
            result.Timestamp, len(result.Data.Res.String()))
    }
}
```

## Best Practices

### 1. Prefer Concurrent Reads

Leverage concurrent reads for performance:

```go
// Good: Concurrent reads
var wg sync.WaitGroup
for _, filter := range filters {
    wg.Add(1)
    go func(f netconf.Filter) {
        defer wg.Done()
        client.Get(ctx, f)
    }(filter)
}
wg.Wait()
```

### 2. Serialize Writes at Application Level

Coordinate write operations explicitly to maintain logical operation order:

```go
// ✅ GOOD: Use channels or mutexes to coordinate writes
writeMutex := &sync.Mutex{}

go func() {
    writeMutex.Lock()
    defer writeMutex.Unlock()

    _, err := client.EditConfig(ctx, "candidate", config1)
    if err != nil {
        log.Printf("Config1 failed: %v", err)
    }
}()

go func() {
    writeMutex.Lock()
    defer writeMutex.Unlock()

    _, err := client.EditConfig(ctx, "candidate", config2)
    if err != nil {
        log.Printf("Config2 failed: %v", err)
    }
}()
```

**Note:** While the library's internal mutex prevents data corruption, application-level coordination ensures:
- Predictable operation order
- Easier debugging and error tracking
- Better control over transaction boundaries
- Clear ownership of the candidate datastore

### 3. Use Context for Cancellation

Pass context for proper cancellation:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
    client.Get(ctx, filter)  // Will be cancelled if ctx is cancelled
}()
```

### 4. Handle Errors Properly

Check errors from concurrent operations:

```go
errChan := make(chan error, numGoroutines)

for i := 0; i < numGoroutines; i++ {
    go func() {
        _, err := client.Get(ctx, filter)
        errChan <- err
    }()
}

for i := 0; i < numGoroutines; i++ {
    if err := <-errChan; err != nil {
        log.Printf("Goroutine error: %v", err)
    }
}
```

### 5. Limit Concurrency

Use semaphores or worker pools:

```go
// Semaphore pattern
sem := make(chan struct{}, 10)  // Limit to 10 concurrent operations

for _, filter := range filters {
    sem <- struct{}{}  // Acquire
    go func(f netconf.Filter) {
        defer func() { <-sem }()  // Release
        client.Get(ctx, f)
    }(filter)
}
```

### 6. Test with Race Detector

Always test concurrent code with race detector:

```bash
go test -race ./...
```

### 7. Use WaitGroups Properly

Ensure all goroutines complete:

```go
var wg sync.WaitGroup

for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        client.Get(ctx, filter)
    }()
}

wg.Wait()  // Wait for all to complete
```

## Common Pitfalls

### Pitfall 1: Concurrent Writes Without Coordination

**Problem:**
```go
// ❌ BAD: Concurrent writes without coordination
go func() { client.EditConfig(ctx, "candidate", config1) }()
go func() { client.EditConfig(ctx, "candidate", config2) }()
// These operations are serialized by library mutex, but logical coordination is lost
// You don't know which config was applied first, making debugging difficult
```

**Why This is Bad:**
- No control over operation order
- Error handling is difficult (which goroutine failed?)
- Race conditions in application logic
- Difficult to debug and reason about

**Solution:**
```go
// ✅ GOOD: Serialize writes explicitly at application level
if _, err := client.Lock(ctx, "candidate"); err != nil {
    return err
}
defer func() {
    if _, err := client.Unlock(ctx, "candidate"); err != nil {
        log.Printf("Warning: Failed to unlock: %v", err)
    }
}()

// Sequential operations with clear order
if _, err := client.EditConfig(ctx, "candidate", config1); err != nil {
    client.Discard(ctx)
    return err
}
if _, err := client.EditConfig(ctx, "candidate", config2); err != nil {
    client.Discard(ctx)
    return err
}
if _, err := client.Commit(ctx); err != nil {
    return err
}
```

### Pitfall 2: Forgetting to Close Clients

**Problem:**
```go
// ❌ BAD: Client leak - creates 100 NETCONF sessions
for i := 0; i < 100; i++ {
    client, _ := netconf.NewClient("192.168.1.1", ...)
    // Missing Close() - each client holds a TCP connection and device session
}
// Result: 100 TCP connections, 100 device sessions, resource exhaustion
```

**Impact:**
- Each unclosed client holds an active TCP connection to the device
- Each unclosed client consumes a device NETCONF session slot (devices typically limit to 10-20 sessions)
- Device may refuse new connections when session limit is reached
- Memory leak in your application
- Potential device resource exhaustion

**Solution:**
```go
// ✅ GOOD: Always close immediately with defer
client, err := netconf.NewClient("192.168.1.1", ...)
if err != nil {
    return err
}
defer client.Close()  // Guarantees cleanup even on panic

// For connection pools, close all clients on shutdown
```

### Pitfall 3: Sharing Client Without Understanding Thread Safety

**Problem:**
```go
// ❌ Misunderstanding: Thinking you need application-level locks for reads
mu := &sync.Mutex{}
go func() {
    mu.Lock()  // Unnecessary - creates false serialization
    defer mu.Unlock()
    client.Get(ctx, filter)
}()
// This defeats the purpose of concurrent reads - operations are now serialized
```

**Why This is Wrong:**
- The library already provides internal RWMutex protection
- Adding external mutex serializes reads unnecessarily
- Significantly reduces throughput for read-heavy workloads
- Creates false bottlenecks in your application

**Solution:**
```go
// ✅ GOOD: No application lock needed for concurrent reads
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        // Library handles synchronization internally with RLock
        res, err := client.Get(ctx, filter)
        if err != nil {
            log.Printf("Goroutine %d failed: %v", id, err)
            return
        }
        log.Printf("Goroutine %d succeeded", id)
    }(i)
}
wg.Wait()
// All 10 reads execute concurrently - maximum throughput
```

### Pitfall 4: Not Handling Context Cancellation

**Problem:**
```go
// ❌ BAD: Goroutine leak - operation never times out or cancels
go func() {
    // Using background context means this goroutine runs until completion
    client.Get(context.Background(), filter)  // Never cancelled
}()
// If network is slow or device hangs, this goroutine runs indefinitely
```

**Impact:**
- Goroutines accumulate over time (leak)
- Cannot cancel long-running operations
- Application may hang during shutdown
- Resource exhaustion in long-running processes
- No way to implement request timeouts

**Solution:**
```go
// ✅ GOOD: Use parent context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

errChan := make(chan error, 1)
go func() {
    _, err := client.Get(ctx, filter)  // Will be cancelled on timeout
    errChan <- err
}()

// Wait for completion or timeout
select {
case err := <-errChan:
    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
case <-ctx.Done():
    return fmt.Errorf("operation timed out: %w", ctx.Err())
}
```

### Pitfall 5: Ignoring Errors from Goroutines

**Problem:**
```go
// ❌ BAD: Error is silently lost
go func() {
    _, err := client.Get(ctx, filter)
    // err is lost - no way to know if operation failed
}()
// Your application continues without knowing if the operation succeeded
```

**Impact:**
- Silent failures that are difficult to debug
- Configuration changes may not be applied
- Monitoring gaps - failed operations not logged
- Data inconsistency
- False sense of success

**Solution 1 - Error Channel:**
```go
// ✅ GOOD: Collect errors via channel
errChan := make(chan error, 1)
go func() {
    _, err := client.Get(ctx, filter)
    errChan <- err
}()

// Wait for result
if err := <-errChan; err != nil {
    log.Printf("Operation failed: %v", err)
    return err
}
```

**Solution 2 - WaitGroup with Error Collection:**
```go
// ✅ GOOD: Collect all errors
var wg sync.WaitGroup
errChan := make(chan error, 10)

for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        _, err := client.Get(ctx, filter)
        if err != nil {
            errChan <- fmt.Errorf("goroutine %d: %w", id, err)
        }
    }(i)
}

// Close error channel when all goroutines complete
go func() {
    wg.Wait()
    close(errChan)
}()

// Collect all errors
var errors []error
for err := range errChan {
    errors = append(errors, err)
    log.Printf("Error: %v", err)
}

if len(errors) > 0 {
    return fmt.Errorf("operations failed: %d errors", len(errors))
}
```

## Concurrency Summary

### Thread-Safety Guarantees

| Operation | Concurrent Safe | Library Protection | Application Coordination |
|-----------|----------------|-------------------|-------------------------|
| `Get()` | ✅ Yes | RLock (shared) | None needed |
| `GetConfig()` | ✅ Yes | RLock (shared) | None needed |
| `Validate()` | ✅ Yes | RLock (shared) | None needed |
| `EditConfig()` | ⚠️ Serialized | Lock (exclusive) | Recommended for order |
| `Lock()` | ⚠️ Serialized | Lock (exclusive) | Required for transaction |
| `Unlock()` | ⚠️ Serialized | Lock (exclusive) | Required for cleanup |
| `Commit()` | ⚠️ Serialized | Lock (exclusive) | Required for transaction |
| `Discard()` | ⚠️ Serialized | Lock (exclusive) | Required for cleanup |
| `CopyConfig()` | ⚠️ Serialized | Lock (exclusive) | Required for transaction |
| `DeleteConfig()` | ⚠️ Serialized | Lock (exclusive) | Required for transaction |
| `RPC()` | ⚠️ Serialized | Lock (exclusive) | Depends on RPC type |

### Performance Considerations

**Read-Heavy Workloads:**
- Excellent concurrency with multiple simultaneous reads
- Scale horizontally with more goroutines
- Limited only by device and network capacity

**Write-Heavy Workloads:**
- Sequential execution due to mutex protection
- Consider batching writes for efficiency
- Use write queues to coordinate multiple writers

**Mixed Workloads:**
- Reads and writes can run concurrently
- Writes block readers briefly during execution
- Design for read-heavy, write-occasional patterns

### Recommended Patterns

**For Monitoring/Polling:**
```go
// ✅ Single client with concurrent read operations
client := createClient()
defer client.Close()

var wg sync.WaitGroup
errChan := make(chan error, len(filters))

for _, filter := range filters {
    wg.Add(1)
    go func(f netconf.Filter) {
        defer wg.Done()
        _, err := client.Get(ctx, f)
        if err != nil {
            errChan <- err
        }
    }(filter)
}

wg.Wait()
close(errChan)

// Check for errors
for err := range errChan {
    log.Printf("Read error: %v", err)
}
```

**For Configuration Management:**
```go
// ✅ Single client with sequential write operations
client := createClient()
defer client.Close()

if _, err := client.Lock(ctx, "candidate"); err != nil {
    return err
}
defer func() {
    if _, err := client.Unlock(ctx, "candidate"); err != nil {
        log.Printf("Warning: unlock failed: %v", err)
    }
}()

// Sequential edits
for _, config := range configs {
    if _, err := client.EditConfig(ctx, "candidate", config); err != nil {
        client.Discard(ctx)
        return err
    }
}
if _, err := client.Commit(ctx); err != nil {
    return err
}
```

**For High-Throughput Applications:**
```go
// ✅ Client pool for independent operations
pool := NewClientPool(host, user, pass, 5)
defer pool.Close()

var wg sync.WaitGroup
errChan := make(chan error, len(tasks))

for _, task := range tasks {
    wg.Add(1)
    go func(t Task) {
        defer wg.Done()
        client := pool.Get()  // Get client from pool
        _, err := client.Get(ctx, t.filter)
        if err != nil {
            errChan <- err
        }
    }(task)
}

go func() {
    wg.Wait()
    close(errChan)
}()

// Check for errors
for err := range errChan {
    log.Printf("Task error: %v", err)
}
```

### Key Takeaways

1. **Read operations are truly concurrent** - No application-level coordination needed
2. **Write operations are automatically serialized** - But you should coordinate at application level for clarity
3. **Always use defer for locks** - Even in concurrent code, lock cleanup is critical
4. **One client can serve many goroutines** - No need to create clients per goroutine for most use cases
5. **Use client pools for isolation** - When operations need complete independence
6. **Test with -race** - Always verify thread safety with race detector
7. **Handle context cancellation** - Prevent goroutine leaks
8. **Collect errors from goroutines** - Never silently ignore errors

### Checklist for Concurrent Code

- ✅ Use single client for read-heavy concurrent operations
- ✅ Use defer with error handling for all locks
- ✅ Coordinate writes at application level (mutex, channels, or sequential)
- ✅ Pass context to all operations for cancellation support
- ✅ Collect and handle errors from all goroutines
- ✅ Use WaitGroup to synchronize goroutine completion
- ✅ Limit concurrency with semaphores or worker pools
- ✅ Close all clients with defer
- ✅ Test with `go test -race`
- ✅ Monitor for goroutine leaks in production

## See Also

- [Quick Start Guide](quickstart.md) - Basic concurrent patterns
- [Operations Guide](operations.md) - Detailed operation documentation with thread-safety notes
- [Filters Guide](filters.md) - Subtree and XPath filter usage
- [Error Handling Guide](error-handling.md) - Error recovery in concurrent code
- [Logging Guide](logging.md) - Structured logging configuration
- [Go Concurrency Patterns](https://go.dev/blog/pipelines) - Official Go concurrency blog
- [Go Memory Model](https://go.dev/ref/mem) - Understanding Go's concurrency guarantees
