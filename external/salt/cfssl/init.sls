/usr/local/src/cfssl-{{ pillar.cfssl.version }}:
  file.managed:
    - source: https://pkg.cfssl.org/R{{ pillar.cfssl.version }}/cfssl_linux-amd64
    - source_hash: {{ pillar.cfssl.hash.cfssl }}
    - mode: 755

/usr/local/bin/cfssl:
  file.symlink:
    - target: /usr/local/src/cfssl-{{ pillar.cfssl.version }}

/usr/local/src/cfssljson-{{ pillar.cfssl.version }}:
  file.managed:
    - source: https://pkg.cfssl.org/R{{ pillar.cfssl.version }}/cfssljson_linux-amd64
    - source_hash: {{ pillar.cfssl.hash.cfssl }}
    - mode: 755

/usr/local/bin/cfssljson:
  file.symlink:
    - target: /usr/local/src/cfssljson-{{ pillar.cfssl.version }}

/etc/ssl/ca:
  file.recurse:
    - source: salt://cfssl/ca

/usr/local/share/ca-certificates/ca.crt:
  file.symlink:
    - target: /etc/ssl/ca/ca.pem
  cmd.watch:
    - name: update-ca-certificates
    - watch:
      - file: /usr/local/share/ca-certificates/ca.crt
