FROM golang AS builder

WORKDIR /app

COPY ./src .

RUN rm go.mod && rm go.sum

RUN go mod init github.com/amaumene/momenarr && go mod tidy

RUN CGO_ENABLED=0 go build -o momenarr

FROM gcr.io/distroless/static:nonroot

COPY --chown=nonroot --from=builder /app/momenarr /app/momenarr

VOLUME /data

EXPOSE 3000/tcp

CMD [ "/app/momenarr" ]
