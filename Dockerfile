FROM golang:1.19-alpine as build

WORKDIR /app
COPY . /app/
RUN go mod download

RUN CGO_ENABLED=0 go build -o /webhook

FROM alpine:3.10 as runtime

COPY --from=build /webhook /usr/local/bin/webhook
RUN chmod +x /usr/local/bin/webhook

ENTRYPOINT ["webhook"]