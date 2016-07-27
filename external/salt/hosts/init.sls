/etc/hosts:
  file.managed:
    - source: salt://hosts/hosts
    - template: jinja
