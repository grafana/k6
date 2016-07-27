grafana/grafana:{{ pillar.grafana.version }}:
  dockerng.image_present

grafana:
  dockerng.running:
    - image: grafana/grafana:{{ pillar.grafana.version }}
    - network_mode: host
    - restart_policy: always
    - environment:
      - GF_AUTH_ANONYMOUS_ENABLED: "True"
      - GF_AUTH_ANONYMOUS_ORG_ROLE: Admin
    - binds:
      - /var/lib/grafana:/var/lib/grafana
    - watch:
      - dockerng: grafana/grafana:{{ pillar.grafana.version }}
