# Build the React web client only. The Emscripten game engine comes from the
# verified upstream Linux release archive copied in the runtime stage.
FROM node:20-bookworm-slim AS web
WORKDIR /src/client
COPY client/package.json client/yarn.lock ./
RUN corepack enable && yarn install --frozen-lockfile
COPY client/ ./
RUN yarn run parcel build --target package \
    && cp src/index.html src/background.png ../pkg/server/static/site/ \
    && mkdir -p ../pkg/server/static/site/game \
    && cp prebuilt-game/game/* ../pkg/server/static/site/game/

FROM golang:1.22-bookworm AS server-build
RUN apt-get update && apt-get install -y --no-install-recommends gcc g++ make swig zlib1g-dev libenet-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/
COPY --from=web /src/pkg/server/static/site/ ./pkg/server/static/site/
RUN CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /out/sour ./cmd/sour

FROM server-build AS verified-server
RUN CGO_ENABLED=1 go test ./pkg/server/servers ./pkg/server/service

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libstdc++6 libenet7 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --home-dir /opt/sour --no-create-home sour \
    && mkdir -p /opt/sour /data/cache && chown -R sour:sour /opt/sour /data
WORKDIR /opt/sour
COPY --from=verified-server --chown=sour:sour /out/sour /usr/local/bin/sour
COPY --chown=sour:sour release/assets/ /opt/sour/assets/
COPY --chown=sour:sour config.yaml /opt/sour/config.yaml
USER sour
EXPOSE 1337/tcp 28785/tcp 28785/udp
ENTRYPOINT ["/usr/local/bin/sour", "serve", "/opt/sour/config.yaml"]
