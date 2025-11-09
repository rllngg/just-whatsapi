#!/bin/bash

# Test Client for WhatsApp Device Manager

BASE_URL="http://localhost:8081"

echo "========================================="
echo "WhatsApp Device Manager Test Client"
echo "========================================="
echo ""

echo "1. Creating a new device..."
RESPONSE=$(curl -s -X POST "$BASE_URL/device/new")
QR_CODE=$(echo "$RESPONSE" | jq -r '.qr_code')

if [ -n "$QR_CODE" ] && [ "$QR_CODE" != "null" ]; then
    echo "✓ Device created successfully!"
    echo "QR Code (base64): ${QR_CODE:0:50}..."
    echo ""
    
    # Decode QR code
    QR_DECODED=$(echo "$QR_CODE" | base64 -d)
    echo "QR Code (decoded): ${QR_DECODED:0:100}..."
    echo ""
else
    echo "✗ Failed to create device"
    echo "Response: $RESPONSE"
    exit 1
fi

echo "2. Getting all devices..."
DEVICES=$(curl -s "$BASE_URL/device")
echo "Devices: $DEVICES"
echo ""

echo "3. Testing invalid method on /device/new (should get 405)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$BASE_URL/device/new")
if [ "$STATUS" == "405" ]; then
    echo "✓ Correctly returned 405 Method Not Allowed"
else
    echo "✗ Expected 405, got $STATUS"
fi
echo ""

echo "========================================="
echo "All tests completed!"
echo "========================================="
