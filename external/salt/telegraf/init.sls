/etc/telegraf.conf:
  file.managed:
    - source: salt://telegraf/telegraf.conf
    - template: jinja

/etc/telegraf.d:
  file.directory

telegraf:{{ pillar.telegraf.version }}:
  dockerng.image_present

telegraf:
  dockerng.running:
    - image: telegraf:{{ pillar.telegraf.version }}
    - cmd: -config /etc/telegraf.conf -config-directory /etc/telegraf.d
    - network_mode: host
    - restart_policy: always
    - binds:
      - /etc/telegraf.conf:/etc/telegraf.conf:ro
      - /etc/telegraf.d:/etc/telegraf.d:ro
    - watch:
      - file: /etc/telegraf.conf
      - dockerng: telegraf:{{ pillar.telegraf.version }}
