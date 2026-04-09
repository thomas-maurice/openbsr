#!/bin/bash
# Bootstrap Garage for the docker-compose.garage.yml deployment.
# Run this ONCE after `docker compose -f deploy/docker-compose.garage.yml up -d`.
# It creates the S3 bucket and API key, then restarts the app with correct credentials.
set -euo pipefail

ADMIN="http://localhost:3903"
TOKEN="s3cr3t"
AUTH="Authorization: Bearer $TOKEN"
COMPOSE="docker compose -f deploy/docker-compose.garage.yml"

echo "=== Waiting for Garage ==="
for i in $(seq 1 30); do
    if curl -sf "$ADMIN/v1/status" -H "$AUTH" > /dev/null 2>&1; then break; fi
    if [ "$i" = "30" ]; then echo "FAIL: Garage not ready"; exit 1; fi
    sleep 1
done

NODE_ID=$(curl -sf "$ADMIN/v1/status" -H "$AUTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['node'])")
echo "Node: ${NODE_ID:0:16}..."

echo "=== Setting up layout ==="
curl -sf -X POST "$ADMIN/v1/layout" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "[{\"id\":\"$NODE_ID\",\"zone\":\"dc1\",\"capacity\":1073741824,\"tags\":[]}]" > /dev/null
LAYOUT_VER=$(curl -sf "$ADMIN/v1/layout" -H "$AUTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['version']+1)")
curl -sf -X POST "$ADMIN/v1/layout/apply" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"version\":$LAYOUT_VER}" > /dev/null

echo "=== Creating bucket ==="
curl -sf -X POST "$ADMIN/v1/bucket" -H "$AUTH" -H 'Content-Type: application/json' \
    -d '{"globalAlias":"openbsr"}' > /dev/null 2>&1 || true
BUCKET_ID=$(curl -sf "$ADMIN/v1/bucket?globalAlias=openbsr" -H "$AUTH" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

echo "=== Creating API key ==="
KEY_RESP=$(curl -sf -X POST "$ADMIN/v1/key" -H "$AUTH" -H 'Content-Type: application/json' \
    -d '{"name":"openbsr-key"}' 2>/dev/null || curl -sf "$ADMIN/v1/key?search=openbsr-key" -H "$AUTH")
ACCESS_KEY=$(echo "$KEY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['accessKeyId'])")
SECRET_KEY=$(echo "$KEY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['secretAccessKey'])")

echo "=== Granting permissions ==="
curl -sf -X POST "$ADMIN/v1/bucket/allow" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"bucketId\":\"$BUCKET_ID\",\"accessKeyId\":\"$ACCESS_KEY\",\"permissions\":{\"read\":true,\"write\":true,\"owner\":true}}" > /dev/null

echo "=== Restarting app with S3 credentials ==="
S3_ACCESS_KEY="$ACCESS_KEY" S3_SECRET_KEY="$SECRET_KEY" $COMPOSE up -d app

echo ""
echo "Done! OpenBSR is running at http://localhost:18080"
echo "S3 Access Key: $ACCESS_KEY"
echo "S3 Secret Key: $SECRET_KEY"
