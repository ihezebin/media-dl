FROM alpine:3.22
ARG BUILD_TAG=unknown
RUN apk add --no-cache ca-certificates ffmpeg tzdata
WORKDIR /app
COPY .package/image/media-dl ./media-dl
COPY webui/dist ./webui/dist
ENV MEDIA_DL_PORT=8080 \
    MEDIA_DL_WEB_DIR=/app/webui/dist
LABEL org.opencontainers.image.version=$BUILD_TAG
EXPOSE 8080
ENTRYPOINT ["/app/media-dl", "server"]
