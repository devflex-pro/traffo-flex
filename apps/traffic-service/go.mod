module github.com/devflex/traffoflex/apps/traffic-service

go 1.22

require (
	github.com/devflex/traffoflex/packages/go-shared v0.0.0
	github.com/go-chi/chi/v5 v5.1.0
)

require (
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/segmentio/kafka-go v0.4.48 // indirect
)

require (
	github.com/klauspost/compress v1.17.6 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.mongodb.org/mongo-driver/v2 v2.6.0
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/text v0.22.0 // indirect
)

replace github.com/devflex/traffoflex/packages/go-shared => ../../packages/go-shared
