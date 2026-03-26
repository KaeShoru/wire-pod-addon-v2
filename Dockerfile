FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    wget \
    git \
    curl \
    build-essential \
    libopus-dev \
    libopusfile-dev \
    pkg-config \
    libssl-dev \
    portaudio19-dev \
    libpulse-dev \
    libasound2-dev \
    libavahi-client-dev \
    libavahi-core-dev \
    avahi-daemon \
    bluez \
    bluez-tools \
    libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

# Install Go for wire-pod
ENV GOLANG_VERSION=1.21
RUN wget -q https://go.dev/dl/go${GOLANG_VERSION}.linux-arm64.tar.gz && \
    tar -C /usr/local -xzf go${GOLANG_VERSION}.linux-arm64.tar.gz && \
    rm go${GOLANG_VERSION}.linux-arm64.tar.gz

ENV PATH=/usr/local/go/bin:$PATH

# Clone and build wire-pod
WORKDIR /app
RUN git clone https://github.com/kercre123/wire-pod.git . && \
    make build && \
    make setup

# Create directories for HA Add-on
RUN mkdir -p /data/wire-pod /data/vector/certs /data/vector/models /var/www/html /etc/nginx

# Copy our custom run.sh
COPY run.sh /

# Make scripts executable
RUN chmod a+x /run.sh

# Expose ports
EXPOSE 8080 8081

# Set working directory
WORKDIR /app/wire-pod/chipper

CMD ["/run.sh"]
