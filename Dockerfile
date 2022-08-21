FROM alpine:latest
LABEL maintainer="aetaric@gmail.com"
COPY checkrr /checkrr
RUN apk add ffmpeg
WORKDIR "/"
ENTRYPOINT [ "/checkrr" ]