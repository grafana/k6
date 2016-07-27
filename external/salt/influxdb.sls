influxdb:{{ pillar.influxdb.version }}:
  dockerng.image_present

influxdb:
  dockerng.running:
    - image: influxdb:{{ pillar.influxdb.version }}
    - network_mode: host
    - restart_policy: always
    - binds:
      - /var/lib/influxdb:/var/lib/influxdb
    - watch:
      - dockerng: influxdb:{{ pillar.influxdb.version }}
