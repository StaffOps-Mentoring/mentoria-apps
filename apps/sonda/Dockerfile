# Caixa de ferramentas para sondar de dentro do cluster: curl, dig, nc.
FROM alpine:3.22
RUN apk add --no-cache curl bind-tools netcat-openbsd
USER 65534:65534
CMD ["sleep", "infinity"]
