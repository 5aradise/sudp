FROM golang:1.25.3-alpine AS build

WORKDIR /app
COPY . .
RUN go build -C e2e/containers/first-message/ -o ../../../cmd

FROM alpine:latest
RUN apk add --no-cache iproute2
WORKDIR /app
COPY --from=build /app/cmd .
COPY e2e/scripts/netem.sh /netem.sh
RUN chmod +x /netem.sh
