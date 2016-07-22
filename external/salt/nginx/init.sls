/srv/www:
  file.recurse:
    - source: salt://nginx/www
    - clean: True

/etc/nginx/nginx.conf:
  file.managed:
    - source: salt://nginx/nginx.conf

/etc/telegraf.d/nginx.conf:
  file.managed:
    - source: salt://nginx/telegraf.conf
    - watch_in:
      - dockerng: telegraf

nginx:
  pkgrepo.managed:
    - name: deb http://nginx.org/packages/ubuntu/ trusty nginx
    - key_url: http://nginx.org/keys/nginx_signing.key
  pkg.installed:
    - require:
      - pkgrepo: nginx
  service.running:
    - enable: True
    - reload: True
    - watch:
      - file: /etc/nginx/nginx.conf
