FROM golang:alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY ./src/ .

RUN rm go.mod && rm go.sum

RUN go mod init github.com/amaumene/momenarr && go mod tidy

RUN CGO_ENABLED=0 go build -o momenarr -ldflags "-w -s" ./cmd/momenarr

FROM scratch

COPY --chown=65532 --from=builder /app/momenarr /app/momenarr
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

VOLUME /data

EXPOSE 3000/tcp

CMD [ "/app/momenarr" ]
