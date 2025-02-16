#!/bin/bash

TIMEOUT=900  # 15 minutes in seconds
LAST_ACTIVITY=$(date +%s)

update_last_activity() {
    LAST_ACTIVITY=$(date +%s)
}

# Monitor both mouse and keyboard events
xinput test-xi2 --root | while read -r line; do
    if [[ $line == *"EVENT"* ]]; then
        update_last_activity
    fi
done &

while true; do
    CURRENT_TIME=$(date +%s)
    IDLE_TIME=$((CURRENT_TIME - LAST_ACTIVITY))
    
    if [ $IDLE_TIME -gt $TIMEOUT ]; then
        echo "No activity detected for 15 minutes. Shutting down container..."
        /usr/bin/docker stop $(hostname)
        exit 0
    fi
    
    sleep 60
done