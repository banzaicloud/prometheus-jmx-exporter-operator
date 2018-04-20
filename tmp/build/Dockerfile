FROM alpine:3.6

ADD tmp/_output/bin/prometheus-jmx-exporter-operator /usr/local/bin/prometheus-jmx-exporter-operator

RUN adduser -D prometheus-jmx-exporter-operator
USER prometheus-jmx-exporter-operator

ADD lib/* /opt/jmx-exporter-loader/
