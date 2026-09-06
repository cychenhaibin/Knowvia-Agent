FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/knowvia-api ./cmd/api

FROM alpine:3.21

RUN addgroup -S knowvia && adduser -S knowvia -G knowvia
COPY --from=build /out/knowvia-api /usr/local/bin/knowvia-api

USER knowvia
EXPOSE 8088
ENTRYPOINT ["/usr/local/bin/knowvia-api"]
