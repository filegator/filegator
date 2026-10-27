# FileGator — one static binary, one small image.
#
#   docker buildx build --platform linux/386,linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64 \
#     -t filegator/filegator:v8.0.0-beta.1 --push .

# Cross-compile on the host architecture, so a multi-platform build never needs
# emulation. CGO_ENABLED=0 makes the binary static, so the libc does not matter.
FROM --platform=$BUILDPLATFORM golang:1.26 AS build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-mod=vendor GOPROXY=off GOTOOLCHAIN=local

# Named paths, not `COPY . .`: private/ and repository/ hold runtime state.
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY main.go ./
COPY internal/ ./internal/
COPY private/users.json.sample ./private/users.json.sample

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} go build \
  -trimpath -buildvcs=false -ldflags "-s -w" -o /out/filegator .

# The runtime filesystem, laid out in the build stage so the final stage needs
# no RUN (a RUN there executes as the target architecture and needs qemu).
RUN mkdir -p /out/root/data/private /out/root/data/repository \
 && cp private/users.json.sample /out/root/data/private/

FROM alpine:3.22

COPY --from=build /out/filegator /usr/local/bin/filegator
COPY --from=build --chown=65532:65532 /out/root/data /data

WORKDIR /data
USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/filegator"]
# -init: no terminal for the first-start question. 0.0.0.0: the default is
# loopback, where a published port reaches nothing.
CMD ["-init", "-addr", "0.0.0.0:8080", "-config", "/data/private/config.json"]
