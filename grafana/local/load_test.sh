#!/bin/bash

# Load test script for PEAS
# Sends 10,000 requests with 75% success rate

set -e

# Configuration
TOTAL_REQUESTS=10000
PEAS_HOST="localhost:10001"
SUCCESS_RATE=75  # Percentage of successful requests

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
successful_count=0
failed_count=0
error_count=0

echo -e "${GREEN}🚀 Starting PEAS load test...${NC}"
echo "📊 Target: $TOTAL_REQUESTS requests"
echo "✅ Success rate: ${SUCCESS_RATE}%"
echo "🎯 Target: $PEAS_HOST"
echo

# Check if environment variables are set
if [[ -z "$GROVE_PORTAL_APP_ID" || -z "$GROVE_PORTAL_API_KEY" ]]; then
    echo -e "${RED}❌ Error: GROVE_PORTAL_APP_ID and GROVE_PORTAL_API_KEY environment variables must be set${NC}"
    echo "Example:"
    echo "export GROVE_PORTAL_APP_ID=your-app-id"
    echo "export GROVE_PORTAL_API_KEY=your-api-key"
    exit 1
fi

# Check if PEAS is running
if ! nc -z localhost 10001 2>/dev/null; then
    echo -e "${RED}❌ Error: PEAS is not running on localhost:10001${NC}"
    echo "Start PEAS with: docker compose up -d"
    exit 1
fi

echo -e "${GREEN}✅ PEAS is running, starting load test...${NC}"
echo

# Function to send successful request
send_success_request() {
    grpcurl -plaintext -d '{
        "attributes": {
            "request": {
                "http": {
                    "path": "/v1/'$GROVE_PORTAL_APP_ID'",
                    "headers": {
                        "Portal-Application-ID": "'$GROVE_PORTAL_APP_ID'",
                        "Authorization": "Bearer '$GROVE_PORTAL_API_KEY'"
                    },
                    "method": "POST"
                }
            }
        }
    }' $PEAS_HOST envoy.service.auth.v3.Authorization/Check >/dev/null 2>&1
}

# Function to send failed request (invalid API key)
send_failed_request() {
    grpcurl -plaintext -d '{
        "attributes": {
            "request": {
                "http": {
                    "path": "/v1/'$GROVE_PORTAL_APP_ID'",
                    "headers": {
                        "Portal-Application-ID": "'$GROVE_PORTAL_APP_ID'",
                        "Authorization": "Bearer invalid-api-key-'$RANDOM'"
                    },
                    "method": "POST"
                }
            }
        }
    }' $PEAS_HOST envoy.service.auth.v3.Authorization/Check >/dev/null 2>&1
}

# Function to send error request (fake app ID)
send_error_request() {
    local fake_app_id="fake-app-$RANDOM"
    grpcurl -plaintext -d '{
        "attributes": {
            "request": {
                "http": {
                    "path": "/v1/'$fake_app_id'",
                    "headers": {
                        "Portal-Application-ID": "'$fake_app_id'",
                        "Authorization": "Bearer fake-api-key-'$RANDOM'"
                    },
                    "method": "POST"
                }
            }
        }
    }' $PEAS_HOST envoy.service.auth.v3.Authorization/Check >/dev/null 2>&1
}

# Function to show progress
show_progress() {
    local current=$1
    local total=$2
    local percent=$((current * 100 / total))
    local bar_length=50
    local filled_length=$((percent * bar_length / 100))
    
    printf "\rProcessed: %d/%d | ✅ Successful: %d | ❌ Failed: %d | ⚠️  Errors: %d" \
        $current $total $successful_count $failed_count $error_count
    # Flush output to ensure it stays on one line
    printf "\r" # Move cursor to start of line
    sleep 0.1 # Add a small delay to allow the buffer to flush
}

# Main load test loop
start_time=$(date +%s)

for ((i=1; i<=TOTAL_REQUESTS; i++)); do
    # Generate random number 1-100 to determine request type
    rand=$((RANDOM % 100 + 1))
    
    if [ $rand -le $SUCCESS_RATE ]; then
        # Send successful request (75% chance)
        if send_success_request; then
            ((successful_count++))
        else
            ((error_count++))
        fi
    elif [ $rand -le 90 ]; then
        # Send failed auth request (15% chance - 75% to 90%)
        if send_failed_request; then
            ((failed_count++))
        else
            ((error_count++))
        fi
    else
        # Send error request (10% chance - 90% to 100%)
        if send_error_request; then
            ((error_count++))
        else
            ((error_count++))
        fi
    fi
    
    # Show progress every 100 requests
    if [ $((i % 100)) -eq 0 ] || [ $i -eq $TOTAL_REQUESTS ]; then
        show_progress $i $TOTAL_REQUESTS
    fi
done

end_time=$(date +%s)
duration=$((end_time - start_time))

echo
echo
echo -e "${GREEN}🎉 Load test completed!${NC}"
echo "⏱️  Duration: ${duration}s"
echo "📈 Requests per second: $((TOTAL_REQUESTS / duration))"
echo "✅ Successful: $successful_count"
echo "❌ Failed auth: $failed_count"
echo "⚠️  Errors: $error_count"
echo
echo -e "${YELLOW}📊 Check your Grafana dashboard at http://localhost:3000${NC}"
echo -e "${YELLOW}📈 Check metrics at http://localhost:9090/metrics${NC}"
