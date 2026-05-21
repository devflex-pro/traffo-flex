FROM golang:1.22-alpine AS build
WORKDIR /src
COPY packages/go-shared ./packages/go-shared
COPY apps/traffic-service ./apps/traffic-service
WORKDIR /src/apps/traffic-service
RUN go mod download
RUN go build -o /out/traffic-service ./cmd/traffic-service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/traffic-service /app/traffic-service
EXPOSE 8080
CMD ["/app/traffic-service"]
