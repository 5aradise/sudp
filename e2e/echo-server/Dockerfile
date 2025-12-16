FROM golang:1.25.3-alpine AS build

WORKDIR /app
COPY . .
RUN go build -C e2e/echo-server/ -o ../../server

FROM alpine:latest
RUN apk add --no-cache iproute2
WORKDIR /app
COPY --from=build /app/server .
COPY e2e/netem.sh /netem.sh
RUN chmod +x /netem.sh
