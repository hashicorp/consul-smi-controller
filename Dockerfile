FROM alpine:latest

RUN mkdir /app
COPY bin/trafficspec /app/trafficspec

CMD /app/trafficspec
