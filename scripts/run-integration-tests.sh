#!/bin/bash
# Integration Test Runner for QuotaLane
# Uses existing docker-compose.yml infrastructure

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 QuotaLane Integration Test Runner${NC}"
echo ""

# Step 1: Check if Docker is running
echo -e "${YELLOW}📋 Step 1: Checking Docker status...${NC}"
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}❌ Docker is not running${NC}"
    echo "Please start Docker/OrbStack and try again"
    exit 1
fi
echo -e "${GREEN}✅ Docker is running${NC}"
echo ""

# Step 2: Ensure MySQL and Redis services are running
echo -e "${YELLOW}📋 Step 2: Starting MySQL and Redis services...${NC}"
docker-compose up -d mysql redis

# Wait for services to be healthy
echo -e "${YELLOW}⏳ Waiting for services to be ready...${NC}"
timeout=60
elapsed=0
while [ $elapsed -lt $timeout ]; do
    mysql_health=$(docker inspect --format='{{.State.Health.Status}}' quotalane-mysql 2>/dev/null || echo "starting")
    redis_health=$(docker inspect --format='{{.State.Health.Status}}' quotalane-redis 2>/dev/null || echo "starting")

    if [ "$mysql_health" = "healthy" ] && [ "$redis_health" = "healthy" ]; then
        echo -e "${GREEN}✅ All services are healthy${NC}"
        break
    fi

    echo "   MySQL: $mysql_health, Redis: $redis_health (${elapsed}s/${timeout}s)"
    sleep 5
    elapsed=$((elapsed + 5))
done

if [ $elapsed -ge $timeout ]; then
    echo -e "${RED}❌ Services failed to start within ${timeout}s${NC}"
    echo ""
    echo "Checking logs:"
    docker-compose logs mysql redis
    exit 1
fi

echo ""

# Step 3: Run integration tests
echo -e "${YELLOW}📋 Step 3: Running integration tests...${NC}"
echo ""

# Set environment variables for tests (use existing docker-compose services)
# Database: quotalane (production database)
# Note: Tests should use transactions and cleanup to avoid polluting production data
export TEST_MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/quotalane?parseTime=true&loc=UTC"
export TEST_REDIS_ADDR="localhost:6379"

# Run tests with integration tag
go test -tags=integration ./internal/biz -v -count=1

test_exit_code=$?

echo ""

# Step 4: Show service status
echo -e "${YELLOW}📋 Docker services status:${NC}"
echo "   MySQL: localhost:3306 (container: quotalane-mysql)"
echo "   Redis: localhost:6379 (container: quotalane-redis)"
echo ""
echo "To stop services: docker-compose stop mysql redis"
echo "To remove all: docker-compose down -v"

echo ""

# Exit with test result
if [ $test_exit_code -eq 0 ]; then
    echo -e "${GREEN}✅ All integration tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Integration tests failed${NC}"
    exit $test_exit_code
fi
