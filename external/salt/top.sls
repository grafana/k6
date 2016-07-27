base:
  '*':
    - common
    - tuning
    - docker
    - telegraf
  
  'role:loadgen':
    - match: grain
    - golang
  
  'role:influx':
    - match: grain
    - influxdb
    - grafana
  
  'role:web':
    - match: grain
    - nginx
