# Tahap Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Salin definisi modul dan unduh dependensi
COPY go.mod go.sum ./
RUN go mod download

# Salin source code
COPY . .

# Build aplikasi dengan optimalisasi
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker-app main.go

# Tahap Final
FROM alpine:3.19

# Tambahkan sertifikat SSL untuk komunikasi API
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Salin binary dari tahap build
COPY --from=builder /worker-app .

# Perintah eksekusi
CMD ["./worker-app"]
