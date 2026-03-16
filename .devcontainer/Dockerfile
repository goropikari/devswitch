FROM mcr.microsoft.com/devcontainers/base:ubuntu24.04

RUN apt-get update && apt-get install -y \
    build-essential \
    curl \
    git \
    ripgrep \
    protobuf-compiler \
	direnv \
    socat \
    nginx \
    neovim

RUN echo 'eval "$(direnv hook bash)"' >> /home/vscode/.bashrc && \
    echo 'alias gst="git status"' >> /home/vscode/.bashrc && \
    echo 'alias gb="git branch"' >> /home/vscode/.bashrc && \
	chown vscode:vscode /home/vscode/.bashrc
