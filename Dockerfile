FROM ghcr.io/kaeshoru/wire-pod-addon:latest

# Create directories for HA Add-on
RUN mkdir -p /data/wire-pod /data/vector/certs /data/vector/models /var/www/html /etc/nginx

# Copy our custom run.sh
COPY run.sh /

# Make scripts executable
RUN chmod a+x /run.sh

EXPOSE 8080 8081

CMD ["/run.sh"]
