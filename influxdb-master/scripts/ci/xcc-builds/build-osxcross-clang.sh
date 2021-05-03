#!/usr/bin/env bash
set -euo pipefail

declare -r SCRIPT_DIR=$(cd $(dirname ${0}) >/dev/null 2>&1 && pwd)
declare -r OUT_DIR=${SCRIPT_DIR}/out

declare -r BUILD_IMAGE=ubuntu:20.04
declare -r OSXCROSS_VERSION=c2ad5e859d12a295c3f686a15bd7181a165bfa82

docker run --rm -i -v ${OUT_DIR}:/out -w /tmp ${BUILD_IMAGE} bash <<EOF
  set -euo pipefail

  declare -r BUILD_TIME=\$(date -u '+%Y%m%d%H%M%S')
  export DEBIAN_FRONTEND=noninteractive

  # Install dependencies.
  apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    clang \
    cmake \
    curl \
    git \
    libssl-dev \
    libxml2-dev \
    llvm-dev \
    lzma-dev \
    patch \
    zlib1g-dev

  # Clone and build osxcross.
  git clone https://github.com/tpoechtrager/osxcross.git /usr/local/osxcross && \
    cd /usr/local/osxcross && \
    git checkout ${OSXCROSS_VERSION} && \
    curl -L -o ./tarballs/MacOSX10.11.sdk.tar.xz https://macos-sdks.s3.amazonaws.com/MacOSX10.11.sdk.tar.xz && \
    UNATTENDED=1 PORTABLE=true OCDEBUG=1 ./build.sh && \
    rm -rf .git build tarballs && \
    cd /tmp

  # Archive the build output.
  cd /usr/local && tar czf /out/osxcross-${OSXCROSS_VERSION}-\${BUILD_TIME}.tar.gz osxcross && cd /tmp
EOF
