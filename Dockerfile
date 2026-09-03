FROM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -X github.com/ashwnn/chain-reaction/internal/buildinfo.Version=${VERSION} -X github.com/ashwnn/chain-reaction/internal/buildinfo.Commit=${COMMIT} -X github.com/ashwnn/chain-reaction/internal/buildinfo.Date=${DATE}" \
    -o /out/chain-reaction \
    ./cmd/chain-reaction

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && adduser -D -u 65532 nonroot

COPY --from=build /out/chain-reaction /usr/local/bin/chain-reaction

USER nonroot

ENTRYPOINT ["chain-reaction"]
CMD ["version"]
