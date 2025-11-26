# Quick Fixes - Immediate Improvements

## 🔴 Critical Fixes (Do First)

### 1. Fix Rate Limiter Thread Safety
**File:** `app/ratelimit/rate_limiter.go`

**Current Code:**
```go
type DeltaRateLimit struct {
    lastTime    time.Time
    lastValue   float64
    interval    time.Duration
    valueChange float64
}
```

**Fixed Code:**
```go
type DeltaRateLimit struct {
    mu          sync.RWMutex
    lastTime    time.Time
    lastValue   float64
    interval    time.Duration
    valueChange float64
}

func (c *DeltaRateLimit) Allow(value float64) bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    if math.Abs(value-c.lastValue) < c.valueChange {
        if now.Sub(c.lastTime) < c.interval {
            return false
        }
    }
    
    c.lastTime = now
    c.lastValue = value
    return true
}
```

### 2. Move Hardcoded Credentials to Config
**File:** `app/wallbox/wallbox.go:298`

**Current:**
```go
w.sqlClient, err = sqlx.Connect("mysql", "root:fJmExsJgmKV7cq8H@tcp(127.0.0.1:3306)/wallbox")
```

**Fixed:**
```go
// Add to WallboxConfig struct in config.go
type WallboxConfig struct {
    Database struct {
        Host     string `ini:"host"`
        Port     int    `ini:"port"`
        User     string `ini:"user"`
        Password string `ini:"password"`
        Database string `ini:"database"`
    } `ini:"database"`
    // ... existing fields
}

// In wallbox.go New()
dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
    config.Database.User,
    config.Database.Password,
    config.Database.Host,
    config.Database.Port,
    config.Database.Database)
w.sqlClient, err = sqlx.Connect("mysql", dsn)
```

### 3. Add Error Handling to String Conversions
**File:** `app/sensors.go:19-26`

**Current:**
```go
func strToInt(val string) int {
    i, _ := strconv.Atoi(val)
    return i
}
```

**Fixed:**
```go
func strToInt(val string) (int, error) {
    return strconv.Atoi(val)
}

// Update callers to handle errors
func strToIntSafe(val string) int {
    i, err := strconv.Atoi(val)
    if err != nil {
        log.Printf("Failed to parse int '%s': %v", val, err)
        return 0
    }
    return i
}
```

### 4. Replace Panic with Graceful Error Handling
**File:** `app/bridge.go:25, 107`

**Current:**
```go
var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
    panic("Connection to MQTT lost")
}
```

**Fixed:**
```go
var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
    log.Printf("ERROR: Connection to MQTT lost: %v", err)
    // Attempt reconnection
    go func() {
        for {
            if token := client.Connect(); token.Wait() && token.Error() == nil {
                log.Println("MQTT reconnected successfully")
                return
            }
            time.Sleep(5 * time.Second)
        }
    }()
}
```

## 🟡 Performance Fixes

### 5. Make MQTT Publishing Async
**File:** `app/bridge.go:227-228`

**Current:**
```go
token := client.Publish(topicPrefix+"/"+key+"/state", 1, true, bytePayload)
token.Wait()
```

**Fixed:**
```go
token := client.Publish(topicPrefix+"/"+key+"/state", 1, true, bytePayload)
go func(key string, token mqtt.Token) {
    token.Wait()
    if token.Error() != nil {
        log.Printf("MQTT publish error for %s: %v", key, token.Error())
    }
}(key, token)
```

### 6. Add Connection Pooling
**File:** `app/wallbox/wallbox.go:298-310`

**Current:**
```go
w.sqlClient, err = sqlx.Connect("mysql", "...")
w.redisClient = redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
})
```

**Fixed:**
```go
w.sqlClient, err = sqlx.Connect("mysql", "...")
if err != nil {
    return nil, fmt.Errorf("connect to database: %w", err)
}

// Configure connection pool
w.sqlClient.SetMaxOpenConns(10)
w.sqlClient.SetMaxIdleConns(5)
w.sqlClient.SetConnMaxLifetime(time.Hour)
w.sqlClient.SetConnMaxIdleTime(10 * time.Minute)

w.redisClient = redis.NewClient(&redis.Options{
    Addr:         "localhost:6379",
    Password:     "",
    DB:           0,
    PoolSize:     10,
    MinIdleConns: 5,
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,
})
```

### 7. Add Context to Database Operations
**File:** `app/wallbox/wallbox.go:331`

**Current:**
```go
func (w *Wallbox) RefreshData() {
    ctx := context.Background()
    // ...
}
```

**Fixed:**
```go
func (w *Wallbox) RefreshData(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
    defer cancel()
    
    stateRes := w.redisClient.HMGet(ctx, "state", getRedisFields(w.Data.RedisState)...)
    if stateRes.Err() != nil {
        return fmt.Errorf("redis HMGet state: %w", stateRes.Err())
    }
    // ... rest of function returns error
}
```

### 8. Batch Redis Operations
**File:** `app/wallbox/wallbox.go:334-350`

**Current:**
```go
stateRes := w.redisClient.HMGet(ctx, "state", ...)
// ... wait ...
m2wRes := w.redisClient.HMGet(ctx, "m2w", ...)
```

**Fixed:**
```go
pipe := w.redisClient.Pipeline()
stateCmd := pipe.HMGet(ctx, "state", getRedisFields(w.Data.RedisState)...)
m2wCmd := pipe.HMGet(ctx, "m2w", getRedisFields(w.Data.RedisM2W)...)

_, err := pipe.Exec(ctx)
if err != nil {
    return fmt.Errorf("redis pipeline: %w", err)
}

if err := stateCmd.Scan(&w.Data.RedisState); err != nil {
    return fmt.Errorf("scan state: %w", err)
}

if err := m2wCmd.Scan(&w.Data.RedisM2W); err != nil {
    return fmt.Errorf("scan m2w: %w", err)
}
```

## 🟢 Code Quality Fixes

### 9. Use Structured Logging
**Add dependency:** `go get github.com/sirupsen/logrus`

**File:** `app/bridge.go` (add at top)
```go
import log "github.com/sirupsen/logrus"

func init() {
    log.SetFormatter(&log.JSONFormatter{})
    log.SetLevel(log.InfoLevel)
}
```

**Replace:**
```go
fmt.Println("Publishing: ", key, payload)
```

**With:**
```go
log.WithFields(log.Fields{
    "key":     key,
    "payload": payload,
    "topic":   topicPrefix + "/" + key + "/state",
}).Debug("Publishing MQTT message")
```

### 10. Extract Magic Numbers to Constants
**File:** `app/wallbox/wallbox.go` (add at top)
```go
const (
    OCPPStatusCacheTTL = 10 * time.Minute
    RedisTimeout       = 500 * time.Millisecond
    SQLTimeout         = 500 * time.Millisecond
    MQTTReconnectDelay = 5 * time.Second
)
```

**Replace:**
```go
if code >= 0 && time.Since(ts) < 10*time.Minute {
```

**With:**
```go
if code >= 0 && time.Since(ts) < OCPPStatusCacheTTL {
```

### 11. Optimize Telemetry Field Mapping
**File:** `app/wallbox/wallbox.go` (add after DataCache struct)
```go
var telemetryFieldMap map[string]int

func init() {
    telemetryFieldMap = make(map[string]int)
    t := reflect.TypeOf(DataCache{}.RedisTelemetry)
    for i := 0; i < t.NumField(); i++ {
        field := t.Field(i)
        redisTag := field.Tag.Get("redis")
        if strings.HasPrefix(redisTag, "telemetry.") {
            sensorID := strings.TrimPrefix(redisTag, "telemetry.")
            telemetryFieldMap[sensorID] = i
        }
    }
}

func (w *Wallbox) updateTelemetryField(sensorID string, value float64) {
    fieldIndex, ok := telemetryFieldMap[sensorID]
    if !ok {
        return // Unknown sensor, ignore silently or log at debug level
    }
    
    v := reflect.ValueOf(&w.Data.RedisTelemetry).Elem()
    if v.Field(fieldIndex).CanSet() {
        v.Field(fieldIndex).SetFloat(value)
        w.HasTelemetry = true
    }
}
```

## 📊 Testing Your Changes

### Before Making Changes
1. Benchmark current performance:
```bash
go test -bench=. -benchmem ./...
```

2. Profile CPU usage:
```bash
go test -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof
```

3. Profile memory:
```bash
go test -memprofile=mem.prof ./...
go tool pprof mem.prof
```

### After Making Changes
1. Run benchmarks again and compare
2. Test with actual Wallbox hardware
3. Monitor MQTT publish latency
4. Check database connection pool usage

## 🎯 Implementation Order

1. **Week 1: Critical Fixes**
   - Fix rate limiter thread safety
   - Move credentials to config
   - Add error handling

2. **Week 2: Performance**
   - Async MQTT publishing
   - Connection pooling
   - Batch Redis operations

3. **Week 3: Code Quality**
   - Structured logging
   - Extract constants
   - Optimize telemetry mapping

4. **Week 4: Testing & Monitoring**
   - Add metrics
   - Performance testing
   - Load testing
