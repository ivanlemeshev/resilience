FROM golang:1.21 AS build-stage
WORKDIR /code
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /service main.go
FROM gcr.io/distroless/base AS build-release-stage
WORKDIR /
COPY --from=build-stage /service /service
EXPOSE 8080
ENTRYPOINT ["/service"]
