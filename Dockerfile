FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
COPY internal/web/ /app/internal/web/
RUN npm run build
# Output is at /app/internal/web/dist/

FROM golang:1-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=1 go build -o bsr ./cmd/bsr

FROM alpine:3
RUN apk add --no-cache ca-certificates
COPY --from=build /app/bsr /bsr
ENTRYPOINT ["/bsr"]
