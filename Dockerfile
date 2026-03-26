FROM ghcr.io/kercre123/wire-pod:main

# Create directories for HA Add-on
RUN mkdir -p /data/wire-pod /data/vector/certs /data/vector/models /var/www/html /etc/nginx

# Copy our custom files
COPY run.sh /
COPY nginx.conf /etc/nginx/nginx.conf
COPY setup-vector.sh /usr/local/bin/

# Make scripts executable
RUN chmod a+x /run.sh /usr/local/bin/setup-vector.sh

EXPOSE 8080 8081

CMD ["/run.sh"]
