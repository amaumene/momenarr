FROM registry.access.redhat.com/ubi9/go-toolset AS builder

COPY ./src/* .

RUN go build -o momenarr

FROM registry.access.redhat.com/ubi9/ubi-minimal

COPY --from=builder /opt/app-root/src/momenarr /app/momenarr

USER 1001

VOLUME /data
EXPOSE 3000/tcp

CMD [ "/app/maumenarr" ]
