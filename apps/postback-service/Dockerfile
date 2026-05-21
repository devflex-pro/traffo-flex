FROM golang:1.22-alpine AS build
WORKDIR /src
COPY packages/go-shared ./packages/go-shared
COPY apps/postback-service ./apps/postback-service
WORKDIR /src/apps/postback-service
RUN go mod download
RUN go build -o /out/postback-service ./cmd/postback-service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/postback-service /app/postback-service
EXPOSE 8081
CMD ["/app/postback-service"]
