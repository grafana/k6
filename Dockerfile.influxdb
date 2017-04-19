FROM influxdb:1.2.2

# Create a k6 db
RUN /bin/bash -c "influxd run & sleep 5 && influx -execute 'CREATE DATABASE k6' && kill %1 && sleep 5"

CMD ["influxd"]