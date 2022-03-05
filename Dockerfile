FROM golang:1.17

RUN apt update \
    && apt dist-upgrade -y \
    && apt install -y chromium

COPY . /opt/bbb/

RUN cd /opt/bbb \
    && go run ./cmd/install \
    && go build -o run .

WORKDIR /opt/bbb
VOLUME /opt/bbb/config
ENTRYPOINT ["/opt/bbb/run"]