FROM node:22.12-bookworm AS web-build
WORKDIR /src/web

RUN apt-get update && apt-get install -y --no-install-recommends \
    brotli \
 && rm -rf /var/lib/apt/lists/*

COPY web/package*.json ./
RUN npm install

COPY web/ ./
COPY src/scripts/brotli-dist.sh /usr/local/bin/brotli-dist
RUN npm run build
RUN chmod +x /usr/local/bin/brotli-dist && /usr/local/bin/brotli-dist /src/web/dist


FROM golang:1.25-bookworm AS go-build
WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
 && rm -rf /var/lib/apt/lists/*

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY integrations/digi4school/go.mod integrations/digi4school/go.sum ./integrations/digi4school/
RUN cd /src/integrations/digi4school && go mod download

COPY src .
COPY integrations ./integrations

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o app
RUN cd /src/integrations/digi4school && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /src/integrations/d4s .


FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    librsvg2-bin \
    ghostscript \
    qpdf \
    webp \
    ca-certificates \
 && rm -rf /var/lib/apt/lists/*


WORKDIR /app

COPY --from=web-build /src/web/dist /app/dist
COPY --from=go-build /src/app /app/app
COPY --from=go-build /src/integrations/d4s /app/d4s
COPY --from=go-build /src/integrations/d4s /app/integrations/d4s
RUN mkdir -p /app/data

EXPOSE 8080
VOLUME ["/app/data"]

ENTRYPOINT ["/app/app"]
