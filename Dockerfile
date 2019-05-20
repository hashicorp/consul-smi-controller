FROM alpine:latest

RUN addgroup -g 1000 -S app && \
    adduser -u 1000 -S app -G app

RUN mkdir /app
COPY bin/smi-controller /app/smi-controller

CMD /app/smi-controller
