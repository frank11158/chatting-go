FROM golang:1.20 AS builder
WORKDIR /go/src/anonymous-chat
COPY . .
RUN make

FROM ubuntu:20.04
WORKDIR /go/src/anonymous-chat
COPY --from=builder /go/src/anonymous-chat/bin/anonymous-chat .
EXPOSE 4000
ENTRYPOINT [ "./anonymous-chat" ]