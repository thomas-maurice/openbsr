#!/bin/bash
# Bootstrap Garage via Admin API.
# Usage: GARAGE_ADMIN=http://localhost:13903 GARAGE_TOKEN=s3cr3t ./setup.sh
set -euo pipefail

ADMIN="${GARAGE_ADMIN:-http://localhost:13903}"
TOKEN="${GARAGE_TOKEN:-s3cr3t}"
AUTH="Authorization: Bearer $TOKEN"

echo "Waiting for Garage admin API..."
for i in $(seq 1 30); do
    if curl -sf "$ADMIN/v1/status" -H "$AUTH" > /dev/null 2>&1; then break; fi
    if [ "$i" = "30" ]; then echo "FAIL: Garage not ready"; exit 1; fi
    sleep 1
done

# Get node ID
NODE_ID=$(curl -sf "$ADMIN/v1/status" -H "$AUTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['node'])")
echo "Node: $NODE_ID"

# Assign layout (zone=dc1, capacity=1GB, tags required by v1 API)
curl -sf -X POST "$ADMIN/v1/layout" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "[{\"id\":\"$NODE_ID\",\"zone\":\"dc1\",\"capacity\":1073741824,\"tags\":[]}]" > /dev/null

# Apply layout
LAYOUT_VER=$(curl -sf "$ADMIN/v1/layout" -H "$AUTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['version']+1)")
curl -sf -X POST "$ADMIN/v1/layout/apply" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"version\":$LAYOUT_VER}" > /dev/null

# Create bucket (ignore if already exists)
curl -sf -X POST "$ADMIN/v1/bucket" -H "$AUTH" -H 'Content-Type: application/json' \
    -d '{"globalAlias":"openbsr"}' > /dev/null 2>&1 || true

# Get bucket ID
BUCKET_ID=$(curl -sf "$ADMIN/v1/bucket?globalAlias=openbsr" -H "$AUTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

# Create API key (ignore if already exists)
KEY_RESP=$(curl -sf -X POST "$ADMIN/v1/key" -H "$AUTH" -H 'Content-Type: application/json' \
    -d '{"name":"openbsr-key"}' 2>/dev/null || curl -sf "$ADMIN/v1/key?search=openbsr-key" -H "$AUTH")
ACCESS_KEY=$(echo "$KEY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['accessKeyId'])")
SECRET_KEY=$(echo "$KEY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['secretAccessKey'])")

# Grant bucket permissions
curl -sf -X POST "$ADMIN/v1/bucket/allow" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"bucketId\":\"$BUCKET_ID\",\"accessKeyId\":\"$ACCESS_KEY\",\"permissions\":{\"read\":true,\"write\":true,\"owner\":true}}" > /dev/null

echo "ACCESS_KEY=$ACCESS_KEY"
echo "SECRET_KEY=$SECRET_KEY"
echo "Garage ready."
