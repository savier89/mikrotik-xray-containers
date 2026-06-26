FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM golang:1.21-alpine AS go-builder
WORKDIR /build
COPY go.mod .
COPY server.go .
COPY --from=builder /app/dist ./dist
RUN CGO_ENABLED=0 go build -o server .

FROM scratch
COPY --from=go-builder /build/server /server
EXPOSE 11501
CMD ["/server"]
