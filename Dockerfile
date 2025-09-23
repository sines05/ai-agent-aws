FROM golang:1.24-alpine AS build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .

# Build both the web application and the MCP server as static binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o web cmd/web/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go

# Moving the binaries to the 'final Image' to make it smaller
FROM alpine
WORKDIR /app
COPY --from=build /build/settings settings
COPY --from=build /build/config.yaml config.yaml
COPY --from=build /build/web .
COPY --from=build /build/server .
CMD ["/app/web"]