FROM alpine:latest
LABEL maintainer="aetaric@gmail.com"
COPY checkrr /checkrr
WORKDIR "/"
ENTRYPOINT [ "/checkrr" ]