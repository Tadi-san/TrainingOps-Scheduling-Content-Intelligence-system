FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY third_party ./third_party
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/trainingops-api ./cmd/api

FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/trainingops-api /app/trainingops-api
EXPOSE 8080
ENTRYPOINT ["/app/trainingops-api"]
