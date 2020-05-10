#!/bin/bash

echo "Testing that bosun starts and stops cleanly"

# Generate a minimal config
echo 'RuleFilePath = "/tmp/rule.conf"' > /tmp/bosun.toml
touch /tmp/rule.conf

# Wait for at most 30 seconds before considering the launch a failure
timeout 30 ./bosun -c /tmp/bosun.toml & TIMEOUT_PID=$!
BOSUN_START_RESULT=$?

# Give Bosun 5 seconds to start, then stop cleanly
sleep 5
kill -SIGINT $TIMEOUT_PID
BOSUN_SIGNAL_RESULT=$?

# Wait for the process to exit
wait $TIMEOUT_PID
TIMEOUT_RESULT=$?

if [ "$BOSUN_START_RESULT" != 0 ]; then
    echo "Failed to start bosun cleanly. Exit code ${BOSUN_START_RESULT}"
fi
if [ "$BOSUN_SIGNAL_RESULT" != 0 ]; then
    echo "Failed to signal bosun to stop cleanly. Likely crashed before signal sent."
fi
if [ "$BOSUN_STOP_RESULT" != 0 ]; then
    echo "Failed to stop bosun cleanly. Exit code ${TIMEOUT_RESULT} (124=60s test timeout reached)"
fi

(( RESULT = BOSUN_START_RESULT | BOSUN_SIGNAL_RESULT | BOSUN_STOP_RESULT ))
exit $RESULT
