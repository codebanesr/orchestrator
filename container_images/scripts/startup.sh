#!/bin/bash

# Start the original startup script in the background
/dockerstartup/startup.sh &

# Start your inactivity monitor script
/usr/local/bin/monitor_inactivity.sh &

# Prevent the script from exiting immediately
wait