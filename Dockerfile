FROM --platform=$BUILDPLATFORM node:22-alpine AS webui-build
WORKDIR /src/webui
COPY webui/package.json webui/yarn.lock ./
RUN corepack enable && yarn install --frozen-lockfile
COPY webui/ ./
RUN yarn build

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS go-build
WORKDIR /src
ARG TARGETOS
ARG TARGETARCH
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=webui-build /src/webui/dist ./webui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/media-dl .

FROM alpine:3.22
ARG BUILD_TAG=unknown
RUN apk add --no-cache ca-certificates ffmpeg tzdata
WORKDIR /app
COPY --from=go-build /out/media-dl ./media-dl
COPY --from=webui-build /src/webui/dist ./webui/dist
RUN mkdir -p /app/downloads
ENV MEDIA_DL_PORT=8080 \
    MEDIA_DL_OUTPUT_DIR=/app/downloads \
    MEDIA_DL_WEB_DIR=/app/webui/dist
LABEL org.opencontainers.image.version=$BUILD_TAG
EXPOSE 8080
ENTRYPOINT ["/app/media-dl", "server"]
