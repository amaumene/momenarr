FROM registry.access.redhat.com/ubi9/go-toolset AS builder

COPY ./src/* .

RUN go build

FROM registry.access.redhat.com/ubi9/ubi-minimal

COPY --from=builder /opt/app-root/src/maumenarr /app/maumenarr

USER 1001

VOLUME /data
EXPOSE 3000/tcp

CMD [ "/app/maumenarr" ]