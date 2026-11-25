# syntax=docker/dockerfile:1.7

ARG TARGET_VERSION=0.17.0
ARG TARGET_LIB_SHA256=a486000f7305cb601e3074eb06026e9eddcf3d7517df7bbc913d2d36e3108121
ARG SOURCE_VERSION=0.13.1
ARG SOURCE_CLI_SHA256=8f7f15fe573c836d2de9ac0ad621de9766f127eb7b745fab195aa868cc5e7c1a

FROM debian:bookworm-slim@sha256:e67eeead736ff7bbe258ef088bff683635c561b1fcfefad0806c1df4ab49ce0e AS target-native
ARG TARGET_VERSION
ARG TARGET_LIB_SHA256
RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates curl \
    && curl --fail --location --output /tmp/liblbug.tar.gz \
       "https://github.com/LadybugDB/ladybug/releases/download/v${TARGET_VERSION}/liblbug-linux-aarch64.tar.gz" \
    && echo "${TARGET_LIB_SHA256}  /tmp/liblbug.tar.gz" | sha256sum --check \
    && mkdir -p /opt/ladybug/lib /opt/ladybug/include \
    && tar --extract --gzip --file /tmp/liblbug.tar.gz --directory /tmp \
    && cp /tmp/lbug.h /opt/ladybug/include/lbug.h \
    && cp --dereference /tmp/liblbug.so.0.17.0 /opt/ladybug/lib/liblbug.so.0.17.0 \
    && ln -s liblbug.so.0.17.0 /opt/ladybug/lib/liblbug.so.0 \
    && ln -s liblbug.so.0 /opt/ladybug/lib/liblbug.so

FROM golang:1.26-bookworm@sha256:ea47ee45f1284d60f9cc09cb2c3bb4f508c256b640bd8bb30342ff13d1adcb52 AS build
WORKDIR /src
RUN apt-get update && apt-get install --yes --no-install-recommends gcc g++
COPY --from=target-native /opt/ladybug /opt/ladybug
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
RUN CGO_ENABLED=1 \
    CGO_CFLAGS="-I/opt/ladybug/include" \
    CGO_LDFLAGS="-L/opt/ladybug/lib -llbug -Wl,-rpath,'$ORIGIN/../lib'" \
    go build -tags system_ladybug -trimpath -ldflags="-s -w" \
    -o /out/ladybug-spike ./cmd/spike

FROM build AS test
RUN CGO_ENABLED=1 \
    CGO_CFLAGS="-I/opt/ladybug/include" \
    CGO_LDFLAGS="-L/opt/ladybug/lib -llbug -Wl,-rpath,/opt/ladybug/lib" \
    go test -tags system_ladybug ./... \
    && CGO_ENABLED=1 \
       CGO_CFLAGS="-I/opt/ladybug/include" \
       CGO_LDFLAGS="-L/opt/ladybug/lib -llbug -Wl,-rpath,/opt/ladybug/lib" \
       go vet -tags system_ladybug ./...

FROM debian:bookworm-slim@sha256:e67eeead736ff7bbe258ef088bff683635c561b1fcfefad0806c1df4ab49ce0e AS runtime
COPY --from=build /out/ladybug-spike /opt/bigbrain/bin/ladybug-spike
COPY --from=target-native /opt/ladybug/lib /opt/bigbrain/lib
ENV LD_LIBRARY_PATH=/opt/bigbrain/lib
ENTRYPOINT ["/opt/bigbrain/bin/ladybug-spike"]

FROM debian:trixie-slim@sha256:8aa5a7bf5a1955bddbe7d74fcdd0b91ab30e74362a8b73af672b0481b35b44a0 AS migration-source
ARG SOURCE_VERSION
ARG SOURCE_CLI_SHA256
RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates curl \
    && curl --fail --location --output /tmp/lbug-cli.tar.gz \
       "https://github.com/LadybugDB/ladybug/releases/download/v${SOURCE_VERSION}/lbug_cli-linux-aarch64.tar.gz" \
    && echo "${SOURCE_CLI_SHA256}  /tmp/lbug-cli.tar.gz" | sha256sum --check \
    && mkdir -p /opt/ladybug \
    && tar --extract --gzip --file /tmp/lbug-cli.tar.gz --directory /opt/ladybug
ENTRYPOINT ["/opt/ladybug/lbug"]

FROM scratch AS package
COPY --from=build /out/ladybug-spike /bigbrain/bin/ladybug-spike
COPY --from=target-native /opt/ladybug/lib /bigbrain/lib
COPY LICENSES /bigbrain/licenses
COPY PACKAGE.md /bigbrain/PACKAGE.md
