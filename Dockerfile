FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/my-cni ./cmd/my-cni/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/install-cni ./cmd/install-cni/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/route-controller ./cmd/route-controller/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/network-policy-controller ./cmd/network-policy-controller/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates iptables
COPY --from=builder /out/my-cni /my-cni
COPY --from=builder /out/install-cni /install-cni
COPY --from=builder /out/route-controller /route-controller
COPY --from=builder /out/network-policy-controller /network-policy-controller
