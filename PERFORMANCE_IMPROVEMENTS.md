# Performance Improvements & Code Quality Analysis

## Executive Summary

This document identifies performance bottlenecks, code quality issues, and optimization opportunities in the wallbox-mqtt-bridge codebase. The analysis covers database operations, Redis interactions, MQTT publishing, concurrency, error handling, and resource management.

---

## 🔴 Critical Performance Issues

### 1. **Synchronous Database Operations in Main Loop**
**Location:** `app/bridge.go:160` - `w.RefreshData()`

**Problem:**
- `RefreshData()` executes synchronous MySQL queries on every polling interval (default: 1 second)
- Blocks the main loop, preventing other operations
- No connection pooling configuration visible
- No prepared statements for repeated queries

**Impact:** High latency, potential connection exhaustion, poor scalability

**Recommendation:**
```go
// Use context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
defer cancel()

// Use prepared statements
// Cache prepared statements at initialization
// Consider async refresh with channel-based updates
```

### 2. **Blocking MQTT Publish Operations**
**Location:** `app/bridge.go:227-228`

**Problem:**
- `token.Wait()` blocks until publish completes
- With many entities, this creates significant latency
- No batching or async publishing

**Impact:** High latency in main loop, delayed sensor updates

**Recommendation:**
```go
// Use async publishing with callback
token := client.Publish(topic, 1, true, bytePayload)
go func() {
    token.Wait()
    if token.Error() != nil {
        log.Printf("MQTT publish error for %s: %v", key, token.Error())
    }
}()
```

### 3. **Inefficient Reflection Usage**
**Location:** `app/wallbox/wallbox.go:948-974` - `updateTelemetryField()`

**Problem:**
- Reflection used on every telemetry event (high frequency)
- No caching of field mappings
- Linear search through struct fields

**Impact:** CPU overhead, especially with many telemetry sensors

**Recommendation:**
```go
// Create a map at initialization
var telemetryFieldMap map[string]reflect.Value

func init() {
    telemetryFieldMap = buildTelemetryFieldMap()
}

// Then use direct map lookup instead of reflection loop
```

### 4. **Multiple Redis HMGet Calls**
**Location:** `app/wallbox/wallbox.go:334-350`

**Problem:**
- Two separate `HMGet` calls that could be batched
- No pipelining
- Synchronous operations

**Impact:** Network round-trips, latency

**Recommendation:**
```go
// Use Redis pipeline or batch HMGet
pipe := w.redisClient.Pipeline()
stateRes := pipe.HMGet(ctx, "state", ...)
m2wRes := pipe.HMGet(ctx, "m2w", ...)
_, err := pipe.Exec(ctx)
```

### 5. **Repeated String Conversions**
**Location:** `app/bridge.go:221` - `fmt.Sprint(payload)`

**Problem:**
- `fmt.Sprint()` called for every entity on every poll
- No caching of string representations
- Allocates memory repeatedly

**Impact:** Memory allocations, GC pressure

**Recommendation:**
```go
// Cache string representations when value hasn't changed
// Use strconv for numeric types instead of fmt.Sprint
```

---

## 🟡 Moderate Performance Issues

### 6. **No Connection Pooling Configuration**
**Location:** `app/wallbox/wallbox.go:298-301`

**Problem:**
- SQL connection created without pool limits
- Redis client created without connection pool settings
- Risk of connection exhaustion under load

**Recommendation:**
```go
// Configure SQL connection pool
db.SetMaxOpenConns(10)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(time.Hour)

// Configure Redis pool
redis.NewClient(&redis.Options{
    PoolSize: 10,
    MinIdleConns: 5,
    // ...
})
```

### 7. **Inefficient Entity Config Serialization**
**Location:** `app/bridge.go:130-132`

**Problem:**
- JSON marshaling happens on every startup
- Could be cached if config doesn't change
- No validation of JSON size

**Recommendation:**
- Cache marshaled configs
- Only remarshal when entity config changes

### 8. **Rate Limiter Not Thread-Safe**
**Location:** `app/ratelimit/rate_limiter.go:22-34`

**Problem:**
- `DeltaRateLimit.Allow()` accessed from multiple goroutines
- No mutex protection
- Race conditions possible

**Impact:** Data races, incorrect rate limiting

**Recommendation:**
```go
type DeltaRateLimit struct {
    mu          sync.RWMutex
    lastTime    time.Time
    lastValue   float64
    // ...
}
```

### 9. **No Context Propagation**
**Location:** Throughout codebase

**Problem:**
- Database and Redis operations don't use context
- No cancellation/timeout support
- Operations can hang indefinitely

**Recommendation:**
- Add context parameter to all I/O operations
- Use context.WithTimeout for all external calls

---

## 🔵 Code Quality Issues

### 10. **Error Handling**
**Locations:** Multiple files

**Problems:**
- `panic()` used for non-fatal errors (`bridge.go:25, 107`)
- Errors ignored (`sensors.go:19-26` - `strToInt`, `strToFloat`)
- No structured error handling

**Recommendation:**
- Replace panics with graceful error handling
- Return errors instead of ignoring them
- Use structured logging with error levels

### 11. **Hardcoded Credentials**
**Location:** `app/wallbox/wallbox.go:298`

**Problem:**
```go
sqlx.Connect("mysql", "root:fJmExsJgmKV7cq8H@tcp(127.0.0.1:3306)/wallbox")
```

**Impact:** Security risk, not configurable

**Recommendation:**
- Move to configuration file
- Use environment variables
- Support credential rotation

### 12. **No Structured Logging**
**Location:** Throughout codebase

**Problem:**
- Mix of `fmt.Println` and `log.Printf`
- No log levels
- No structured fields

**Recommendation:**
- Use structured logger (e.g., `logrus`, `zap`)
- Consistent logging format
- Log levels (DEBUG, INFO, WARN, ERROR)

### 13. **Magic Numbers and Strings**
**Location:** Throughout codebase

**Examples:**
- `10*time.Minute` in `getTelemetryOCPPStatus()`
- Hardcoded timeouts
- Magic status codes

**Recommendation:**
- Extract to constants
- Make configurable where appropriate

### 14. **No Metrics/Monitoring**
**Location:** N/A

**Problem:**
- No performance metrics
- No health checks
- No observability

**Recommendation:**
- Add Prometheus metrics
- Track: publish latency, DB query time, Redis latency, error rates
- Health check endpoint

---

## 🟢 Optimization Opportunities

### 15. **Batch MQTT Publishing**
**Current:** Individual publish per entity
**Optimization:** Batch multiple updates in single message or use MQTT 5.0 batching

### 16. **Cache Entity Values**
**Current:** Re-serialize every poll
**Optimization:** Only serialize when value changes (already partially done with `published` map)

### 17. **Use Worker Pool for MQTT**
**Current:** Sequential publishing
**Optimization:** Worker pool for async MQTT operations

### 18. **Optimize SQL Queries**
**Location:** `app/wallbox/wallbox.go:352-368`

**Problem:**
- Complex query with multiple JOINs
- Could be optimized with indexes
- No query result caching

**Recommendation:**
- Add database indexes
- Consider materialized views for frequently accessed data
- Cache query results with TTL

### 19. **Reduce Memory Allocations**
**Location:** Multiple locations

**Optimizations:**
- Reuse byte buffers for MQTT payloads
- Pool JSON encoders/decoders
- Use `sync.Pool` for frequently allocated objects

### 20. **Parallel Data Refresh**
**Current:** Sequential Redis and SQL queries
**Optimization:** Use goroutines to fetch Redis and SQL data in parallel

---

## 📊 Performance Metrics to Track

1. **Database Query Latency** - P50, P95, P99
2. **Redis Operation Latency** - HMGet, Subscribe
3. **MQTT Publish Latency** - Per message, batch
4. **Main Loop Duration** - Time per polling cycle
5. **Memory Usage** - Heap, allocations per second
6. **Goroutine Count** - Active goroutines
7. **Error Rates** - By operation type

---

## 🎯 Priority Recommendations

### High Priority (Immediate Impact)
1. ✅ Fix rate limiter thread-safety
2. ✅ Add context/timeout to DB operations
3. ✅ Make MQTT publishing async
4. ✅ Optimize reflection usage in telemetry processing
5. ✅ Move hardcoded credentials to config

### Medium Priority (Significant Improvement)
6. ✅ Add connection pooling configuration
7. ✅ Batch Redis operations
8. ✅ Implement structured logging
9. ✅ Add error handling (remove panics)
10. ✅ Parallel data refresh

### Low Priority (Nice to Have)
11. ✅ Add metrics/monitoring
12. ✅ Optimize SQL queries
13. ✅ Reduce memory allocations
14. ✅ Add health check endpoint

---

## 🔧 Implementation Examples

### Example 1: Async MQTT Publishing
```go
type MQTTPublisher struct {
    client mqtt.Client
    queue  chan publishJob
    wg     sync.WaitGroup
}

type publishJob struct {
    topic   string
    payload []byte
    qos     byte
    retain  bool
}

func (p *MQTTPublisher) Start(workers int) {
    for i := 0; i < workers; i++ {
        p.wg.Add(1)
        go p.worker()
    }
}

func (p *MQTTPublisher) worker() {
    defer p.wg.Done()
    for job := range p.queue {
        token := p.client.Publish(job.topic, job.qos, job.retain, job.payload)
        if token.Wait() && token.Error() != nil {
            log.Printf("MQTT publish error: %v", token.Error())
        }
    }
}
```

### Example 2: Optimized Telemetry Field Mapping
```go
var telemetryFieldMap map[string]*fieldInfo

type fieldInfo struct {
    value *reflect.Value
    index int
}

func initTelemetryFieldMap() {
    telemetryFieldMap = make(map[string]*fieldInfo)
    v := reflect.ValueOf(&RedisTelemetry{}).Elem()
    t := v.Type()
    
    for i := 0; i < v.NumField(); i++ {
        field := t.Field(i)
        redisTag := field.Tag.Get("redis")
        if strings.HasPrefix(redisTag, "telemetry.") {
            sensorID := strings.TrimPrefix(redisTag, "telemetry.")
            telemetryFieldMap[sensorID] = &fieldInfo{
                index: i,
            }
        }
    }
}

func (w *Wallbox) updateTelemetryFieldFast(sensorID string, value float64) {
    info, ok := telemetryFieldMap[sensorID]
    if !ok {
        return // Unknown sensor, ignore
    }
    
    v := reflect.ValueOf(&w.Data.RedisTelemetry).Elem()
    v.Field(info.index).SetFloat(value)
    w.HasTelemetry = true
}
```

### Example 3: Batched Redis Operations
```go
func (w *Wallbox) RefreshDataBatched(ctx context.Context) error {
    pipe := w.redisClient.Pipeline()
    
    stateCmd := pipe.HMGet(ctx, "state", getRedisFields(w.Data.RedisState)...)
    m2wCmd := pipe.HMGet(ctx, "m2w", getRedisFields(w.Data.RedisM2W)...)
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("redis pipeline exec: %w", err)
    }
    
    if err := stateCmd.Scan(&w.Data.RedisState); err != nil {
        return fmt.Errorf("scan state: %w", err)
    }
    
    if err := m2wCmd.Scan(&w.Data.RedisM2W); err != nil {
        return fmt.Errorf("scan m2w: %w", err)
    }
    
    // SQL query in parallel
    // ...
    return nil
}
```

---

## 📈 Expected Performance Gains

| Optimization | Expected Improvement |
|-------------|---------------------|
| Async MQTT Publishing | 50-80% reduction in main loop latency |
| Batched Redis Operations | 30-50% reduction in Redis latency |
| Optimized Reflection | 20-40% reduction in CPU usage for telemetry |
| Connection Pooling | Prevents connection exhaustion, 10-20% improvement |
| Parallel Data Refresh | 30-50% reduction in refresh time |
| Prepared Statements | 10-15% reduction in SQL query time |

---

## 🧪 Testing Recommendations

1. **Load Testing**
   - Test with high-frequency telemetry events
   - Test with many concurrent MQTT subscribers
   - Test database under load

2. **Stress Testing**
   - Redis connection failures
   - Database connection failures
   - MQTT broker disconnections

3. **Performance Profiling**
   - Use `pprof` to identify bottlenecks
   - Profile memory allocations
   - Profile CPU usage

4. **Integration Testing**
   - Test with actual Wallbox hardware
   - Test with Home Assistant integration
   - Test OCPP mismatch detection

---

## 📝 Additional Notes

- Consider using `context.Context` throughout for cancellation
- Add graceful shutdown handling
- Consider using `sync.Map` for concurrent read-heavy maps
- Evaluate using `go-redis` pipeline for batch operations
- Consider implementing a circuit breaker for external services
- Add retry logic with exponential backoff for transient failures
