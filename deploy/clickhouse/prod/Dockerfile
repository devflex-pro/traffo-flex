FROM clickhouse/clickhouse-server:24.8

COPY deploy/clickhouse/prod/cluster.xml /etc/clickhouse-server/config.d/traffoflex-cluster.xml
COPY deploy/clickhouse/prod/render-schema.sh /usr/local/bin/render-traffoflex-schema
COPY deploy/clickhouse/init/ /tmp/traffoflex-schema/
RUN /bin/sh /usr/local/bin/render-traffoflex-schema
