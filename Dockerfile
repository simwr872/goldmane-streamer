FROM golang:1.25 AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go generate
RUN CGO_ENABLED=0 go build -o /goldmane-streamer main.go

FROM gcr.io/distroless/static
COPY --from=build /goldmane-streamer /goldmane-streamer
ENTRYPOINT ["/goldmane-streamer"]
