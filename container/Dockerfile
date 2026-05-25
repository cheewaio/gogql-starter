## -----------------------------------------------------
## Build stage: compiles the production binary
FROM dhi.io/golang:1.26.0-alpine3.23-dev AS build-stage

ARG PROJECT_DIR=/app
ENV PROJECT_DIR=$PROJECT_DIR
WORKDIR $PROJECT_DIR

# Cache and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build the binary
COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags='-s -w' -o ./main cmd/main.go

## -----------------------------------------------------
## Dev stage: hot-reload + remote debugging
FROM dhi.io/golang:1.26.0-alpine3.23-dev AS dev-stage

ARG PROJECT_DIR=/app
ENV PROJECT_DIR=$PROJECT_DIR
WORKDIR $PROJECT_DIR

RUN apk add --no-cache git bash procps build-base linux-headers
ENV PATH="/go/bin:${PATH}"
RUN go install github.com/air-verse/air@latest
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# copy source for mounted development use
COPY . .

EXPOSE 4000 4444

# copy entrypoint and air config
COPY container/entrypoint.sh /entrypoint.sh
COPY .air.toml ${PROJECT_DIR}/.air.toml
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

## -----------------------------------------------------
## Runtime stage: lean production image
FROM dhi.io/golang:1.26.0-alpine3.23 AS runtime-stage

ENV PORT=4000

# dev-stage baked in PROJECT_DIR, copy binary from there
COPY --from=dev-stage /entrypoint.sh /entrypoint.sh

ARG PROJECT_DIR=/app
COPY --from=build-stage ${PROJECT_DIR}/main /app/main
COPY --from=build-stage ${PROJECT_DIR}/database/migrations /app/database/migrations
COPY --from=build-stage ${PROJECT_DIR}/graph/*.graphql /app/graph/

WORKDIR /app

EXPOSE ${PORT}

ENTRYPOINT ["/app/main"]
