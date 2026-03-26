# Use official Python base image
FROM python:3.11-slim

# Install minimal system dependencies
RUN apt-get update && apt-get install -y \
    wget \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Download pre-built wire-pod release (avoid compilation)
RUN wget -q https://github.com/kercre123/wire-pod/releases/latest/download/wire-pod-linux-arm64.tar.gz -O /tmp/wire-pod.tar.gz && \
    mkdir -p /app && \
    tar -xzf /tmp/wire-pod.tar.gz -C /app && \
    rm /tmp/wire-pod.tar.gz

# Create directories for HA Add-on
RUN mkdir -p /data/wire-pod /data/vector/certs /data/vector/models /var/www/html /etc/nginx

# Copy our custom run.sh
COPY run.sh /

# Make scripts executable
RUN chmod a+x /run.sh

# Expose ports
EXPOSE 8080 8081

# Set working directory
WORKDIR /app

CMD ["/run.sh"]
