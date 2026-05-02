FROM ubuntu:24.04

# Install dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    openssl \
    libatomic1 \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user
RUN useradd -m -s /bin/bash appuser

# Switch to the new user
USER appuser
WORKDIR /home/appuser/workspace

# Install mise
RUN curl https://mise.run | sh

# Add mise and its shims to PATH
ENV PATH="/home/appuser/.local/share/mise/shims:/home/appuser/.local/bin:${PATH}"

# Create mise config directory
RUN mkdir -p /home/appuser/.config/mise

# Copy mise.toml and install tools
COPY --chown=appuser:appuser mise.toml /home/appuser/.config/mise/config.toml 
RUN mise trust && mise install

# Generate a self-signed TLS certificate (required when binding to 0.0.0.0)
RUN openssl req -x509 -newkey rsa:4096 -keyout /home/appuser/key.pem \
    -out /home/appuser/cert.pem -days 3650 -nodes \
    -subj "/CN=localhost"

# Expose zellij web port
EXPOSE 8082



