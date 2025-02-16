FROM accetto/ubuntu-vnc-xfce-g3

# Copy the inactivity monitor script
COPY ../scripts/monitor_inactivity.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/monitor_inactivity.sh

# Copy your custom startup wrapper
COPY ../scripts/startup.sh /startup.sh
RUN chmod +x /startup.sh

# Install xinput for monitoring user activity
RUN apt-get update && apt-get install -y xinput && rm -rf /var/lib/apt/lists/*

# Ensure `tini` still manages the process
CMD ["/startup.sh"]