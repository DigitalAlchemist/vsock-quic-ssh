all: vendor patch agent client

agent:
	go build -o agent cmd/agent/main.go

client:
	go build -o client cmd/client/main.go
 
vendor:
	go mod vendor

patch: vendor
	patch -p1 -d vendor/ < 0001-patch-lucas-clemente-quic-go.patch
