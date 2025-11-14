# Integration Tests for Account Refresh

## Prerequisites

### 1. MySQL Database
You need a running MySQL instance for integration tests:

```bash
# Using Docker
docker run -d \
  --name quotalane-test-mysql \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=quotalane_test \
  -p 3306:3306 \
  mysql:8.0

# Or using Docker Compose (create docker-compose.test.yml):
version: '3.8'
services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: quotalane_test
    ports:
      - "3306:3306"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

### 2. Redis
You need a running Redis instance:

```bash
# Using Docker
docker run -d \
  --name quotalane-test-redis \
  -p 6379:6379 \
  redis:7-alpine
```

## Running Integration Tests

### 1. Start Infrastructure

```bash
# If using docker-compose.test.yml
docker-compose -f docker-compose.test.yml up -d

# Wait for services to be ready
sleep 5
```

### 2. Run Tests

```bash
# From QuotaLane directory
go test -tags=integration ./internal/biz -v

# Or run specific test
go test -tags=integration ./internal/biz -v -run TestRefreshClaudeToken_Success

# Run with coverage
go test -tags=integration ./internal/biz -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 3. Cleanup

```bash
# Stop and remove containers
docker-compose -f docker-compose.test.yml down -v

# Or if using individual containers:
docker stop quotalane-test-mysql quotalane-test-redis
docker rm quotalane-test-mysql quotalane-test-redis
```

## Test Coverage

The integration test suite covers:

### 1. Single Token Refresh (`TestRefreshClaudeToken_Success`)
- ✅ Decrypt existing OAuth data
- ✅ Call OAuth service to refresh token
- ✅ Encrypt new OAuth data
- ✅ Update database with new token and expires_at
- ✅ Reset health score to 100
- ✅ Clear Redis failure counter

### 2. Refresh Failure Handling (`TestRefreshClaudeToken_Failure`)
- ✅ OAuth service returns error
- ✅ Health score decreased by 20
- ✅ Redis failure counter incremented
- ✅ TTL set to 30 minutes on failure counter

### 3. Consecutive Failures (`TestRefreshClaudeToken_ConsecutiveFailures`)
- ✅ 3 consecutive refresh failures
- ✅ Account marked as ERROR status after 3rd failure
- ✅ Health score decreases correctly (100 → 80 → 60 → 40)
- ✅ Alert marker set in Redis with 24-hour TTL

### 4. Batch Auto Refresh (`TestAutoRefreshTokens_BatchProcessing`)
- ✅ Create 10 expiring accounts
- ✅ Concurrent refresh (max 5 concurrent workers)
- ✅ All accounts refreshed successfully
- ✅ Verify concurrent execution is faster than sequential
- ✅ Verify all accounts updated in database

### 5. Partial Batch Failures (`TestAutoRefreshTokens_PartialFailures`)
- ✅ Batch refresh with some successes and some failures
- ✅ Overall operation succeeds (partial success acceptable)
- ✅ Failed accounts have health score decreased
- ✅ Successful accounts have tokens updated

### 6. Query Filtering (`TestListExpiringAccounts`)
- ✅ Only returns Claude accounts (claude-official, claude-console)
- ✅ Only returns active accounts
- ✅ Only returns accounts with oauth_expires_at <= threshold
- ✅ Filters out non-Claude providers (Gemini, OpenAI, etc.)
- ✅ Filters out inactive/error accounts
- ✅ Filters out accounts without OAuth data

## Configuration

### Database Connection
Default DSN: `root:root@tcp(127.0.0.1:3306)/quotalane_test?parseTime=true&loc=UTC`

To customize, modify `setupTestSuite()` in the test file:
```go
dsn := "your_user:your_password@tcp(host:port)/dbname?parseTime=true&loc=UTC"
```

### Redis Connection
Default: `localhost:6379`, DB 1

To customize:
```go
rdb := redis.NewClient(&redis.Options{
    Addr: "your_host:6379",
    DB:   1,
})
```

### Encryption Key
Default test key: `12345678901234567890123456789012` (32 bytes)

**Warning**: This is for testing only. Production should use a secure random key.

## Troubleshooting

### MySQL Connection Failed
```
Error: Failed to connect to MySQL
```
**Solution**:
- Check MySQL is running: `docker ps | grep mysql`
- Verify port 3306 is accessible: `nc -zv localhost 3306`
- Check credentials in DSN string

### Redis Connection Failed
```
Error: Failed to connect to Redis
```
**Solution**:
- Check Redis is running: `docker ps | grep redis`
- Verify port 6379 is accessible: `nc -zv localhost 6379`
- Check Redis is accepting connections: `redis-cli ping`

### Schema Migration Failed
```
Error: Failed to migrate schema
```
**Solution**:
- Ensure database exists: `mysql -u root -proot -e "CREATE DATABASE IF NOT EXISTS quotalane_test;"`
- Check GORM AutoMigrate logs for specific error

### Tests Hang or Timeout
**Solution**:
- Check for deadlocks in concurrent tests
- Verify Redis is not blocking (check `SLOWLOG`)
- Increase test timeout: `go test -timeout 5m -tags=integration ./internal/biz`

## CI/CD Integration

Add to `.github/workflows/test.yml`:

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration-test:
    runs-on: ubuntu-latest

    services:
      mysql:
        image: mysql:8.0
        env:
          MYSQL_ROOT_PASSWORD: root
          MYSQL_DATABASE: quotalane_test
        ports:
          - 3306:3306
        options: >-
          --health-cmd="mysqladmin ping"
          --health-interval=10s
          --health-timeout=5s
          --health-retries=5

      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
        options: >-
          --health-cmd="redis-cli ping"
          --health-interval=10s
          --health-timeout=5s
          --health-retries=5

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'

      - name: Wait for MySQL
        run: |
          for i in {1..30}; do
            mysqladmin ping -h 127.0.0.1 -u root -proot && break
            echo "Waiting for MySQL..."
            sleep 2
          done

      - name: Run Integration Tests
        run: go test -tags=integration ./internal/biz -v
        working-directory: ./QuotaLane
```

## Performance Benchmarks

Expected performance on local environment:
- Single token refresh: ~200-300ms (including encryption)
- Batch refresh (10 accounts, 5 concurrent): ~400-600ms
- Sequential would be: ~2-3 seconds (5x slower)

## Notes

- All tests use `// +build integration` tag to avoid running in unit test suite
- Tests automatically clean up database and Redis after each run
- Mock HTTP server is used to simulate OAuth responses
- Tests verify both success and failure scenarios
- Concurrent tests verify semaphore-based rate limiting (max 5 concurrent)
