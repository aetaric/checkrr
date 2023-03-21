FROM alpine:latest
LABEL maintainer="aetaric@gmail.com"
COPY checkrr /checkrr
RUN apk add ffmpeg
RUN apk add tzdata
WORKDIR "/"
ENTRYPOINT [ "/checkrr" ]