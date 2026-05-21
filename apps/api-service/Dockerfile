FROM golang:1.22-alpine AS build
WORKDIR /src
COPY packages/go-shared ./packages/go-shared
COPY apps/api-service ./apps/api-service
WORKDIR /src/apps/api-service
RUN go mod download
RUN go build -o /out/api-service ./cmd/api-service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/api-service /app/api-service
EXPOSE 8070
CMD ["/app/api-service"]
