FROM node:12.16.1-alpine3.11 as nodejs-builder
RUN mkdir -p /src/ui
COPY ui/package.json ui/package-lock.json /src/ui/
RUN cd /src/ui && npm install
RUN apk add make git
COPY Makefile /src/Makefile
COPY ui /src/ui
RUN make -C /src ui

FROM golang:1.14.0-alpine3.11 as go-builder
RUN apk add make git
COPY Makefile /src/Makefile
COPY go.mod /src/go.mod
COPY go.sum /src/go.sum
RUN make -C /src download-deps
COPY --from=nodejs-builder /src/ui /src/ui
COPY --from=nodejs-builder /src/.build /src/.build
COPY cmd /src/cmd
COPY internal /src/internal
ARG VERSION
RUN CGO_ENABLED=0 make -C /src VERSION="${VERSION:-dev}" karma

FROM alpine:3.11
COPY --from=lmierzwa/kthxbye:v0.6 /kthxbye /kthxbye
COPY --from=prom/alertmanager:v0.20.0 /bin/alertmanager /alertmanager
RUN apk add supervisor python && rm  -rf /tmp/* /var/cache/apk/*
COPY demo/supervisord.conf /etc/supervisord.conf
COPY demo/alertmanager.yaml /etc/alertmanager.yaml
COPY demo/generator.py /generator.py
COPY --from=go-builder /src/karma /karma
COPY demo/karma.yaml /etc/karma.yaml
COPY demo/acls.yaml /etc/acls.yaml
COPY demo/custom.js /custom.js
RUN adduser -D karma
USER karma
ENV GOGC=20
CMD supervisord --nodaemon --configuration /etc/supervisord.conf
