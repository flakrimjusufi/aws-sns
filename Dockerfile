# syntax=docker/dockerfile:1

FROM golang:1.18.1-alpine

WORKDIR /

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /main

CMD [ "/main" ]