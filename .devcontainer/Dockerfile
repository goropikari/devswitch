FROM mcr.microsoft.com/devcontainers/base:ubuntu24.04

RUN apt-get update && apt-get install -y \
    build-essential \
    curl \
    git \
    ripgrep \
    protobuf-compiler \
	direnv

RUN echo 'eval "$(direnv hook bash)"' >> /home/vscode/.bashrc && \
	chown vscode:vscode /home/vscode/.bashrc
