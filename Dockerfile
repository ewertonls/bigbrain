# syntax=docker/dockerfile:1.7

ARG TARGET_VERSION=0.17.0
ARG TARGET_LIB_SHA256=a486000f7305cb601e3074eb06026e9eddcf3d7517df7bbc913d2d36e3108121

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
COPY internal ./internal
ENV CGO_ENABLED=1 \
    CGO_CFLAGS=-I/opt/ladybug/include \
    LD_LIBRARY_PATH=/opt/ladybug/lib
RUN CGO_LDFLAGS="-L/opt/ladybug/lib -llbug -Wl,-rpath,\$ORIGIN/../lib" \
    go build -tags system_ladybug -trimpath -ldflags="-s -w" -o /out/bin/bigbrain ./cmd/bigbrain

FROM build AS test
RUN CGO_LDFLAGS="-L/opt/ladybug/lib -llbug" \
    go test -tags system_ladybug ./... \
    && CGO_LDFLAGS="-L/opt/ladybug/lib -llbug" \
    go vet -tags system_ladybug ./...

FROM scratch AS package
COPY --from=build /out/bin/bigbrain /bigbrain/bin/bigbrain
COPY --from=target-native /opt/ladybug/lib /bigbrain/lib
COPY spikes/ladybug/LICENSES /bigbrain/licenses
COPY README.md /bigbrain/README.md
COPY integrations /bigbrain/integrations

FROM build AS acceptance-build
COPY tests/acceptance ./tests/acceptance
RUN export CGO_LDFLAGS="-L/opt/ladybug/lib -llbug -Wl,-rpath,\$ORIGIN/../lib" \
    && go build -tags system_ladybug,foundation_acceptance -trimpath -o /out/bin/bigbrain-faults ./cmd/bigbrain \
    && go build -tags system_ladybug -trimpath -o /out/bin/acceptance ./tests/acceptance

FROM debian:bookworm-slim@sha256:e67eeead736ff7bbe258ef088bff683635c561b1fcfefad0806c1df4ab49ce0e AS acceptance
RUN apt-get update \
    && apt-get install --yes --no-install-recommends git python3 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=acceptance-build /out/bin/bigbrain-faults /opt/acceptance/bin/bigbrain-faults
COPY --from=acceptance-build /out/bin/acceptance /opt/acceptance/bin/acceptance
COPY --from=target-native /opt/ladybug/lib /opt/acceptance/lib
ENTRYPOINT ["/opt/acceptance/bin/acceptance"]
