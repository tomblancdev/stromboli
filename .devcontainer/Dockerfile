# Stromboli Development Container
FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Install Podman (for container management)
RUN apt-get update && apt-get install -y --no-install-recommends \
    podman \
    buildah \
    skopeo \
    make \
    curl \
    jq \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Go will be installed via devcontainer features
