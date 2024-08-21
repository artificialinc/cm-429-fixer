FROM golang:1.22 as build_image

ENV CI=true

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.60.1

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY cmd ./cmd
COPY pkg ./pkg
COPY .golangci.yaml ./

ENV GOLANGCI_LINT_CACHE=/root/.cache/golangci-lint
RUN --mount=type=cache,target=/root/.cache/golangci-lint golangci-lint run
RUN go test -race -count=2 ./...
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -o ./bin/fixer ./cmd/fixer

FROM busybox

COPY --from=build_image /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build_image /app/bin/fixer /usr/local/bin/

CMD fixer
