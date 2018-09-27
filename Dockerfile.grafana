FROM grafana/grafana:5.2.4

USER root

RUN apt-get update && apt-get -y install curl


# Change the default data directory (otherwise grafana.db won't persist)
RUN mkdir /var/lib/grafanadb
ENV GF_PATHS_DATA /var/lib/grafanadb

# Init Grafana sqlite db and preconfigure our data source to be our influxdb k6 db
RUN bash -c '/run.sh & sleep 15 && curl -s -H "Content-Type: application/json" -X POST \
    --data '"'"'{"name": "myinfluxdb", "type": "influxdb", "access": "proxy", "url": "http://influxdb:8086", "database": "k6", "isDefault": true}'"'"' \
    http://admin:admin@localhost:3000/api/datasources \
    && kill -SIGINT %%'


CMD ["/run.sh"]
