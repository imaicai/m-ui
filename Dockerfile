FROM golang:latest AS builder
WORKDIR /root
COPY . .
RUN go build -o x-ui main.go


FROM debian:11-slim
RUN apt-get update && apt-get install -y --no-install-recommends -y ca-certificates \
    && apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
WORKDIR /root
COPY --from=builder  /root/x-ui /root/x-ui
COPY bin/. /root/bin/.
VOLUME [ "/etc/m-ui" ]
CMD [ "./x-ui" ]
