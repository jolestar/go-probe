FROM alpine:3.6
MAINTAINER jolestar <jolestar@gmail.com>

RUN apk add --no-cache curl bash tcpdump nmap nmap-nping
RUN curl -L https://github.com/sequenceiq/docker-alpine-dig/releases/download/v9.10.2/dig.tgz|tar -xzv -C /usr/local/bin/

COPY bin/alpine/go-probe /usr/bin/

EXPOSE 80

ENTRYPOINT ["/usr/bin/go-probe"]
