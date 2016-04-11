nginx:
  pkgrepo.managed:
    - name: deb http://nginx.org/packages/ubuntu/ trusty nginx
    - key_url: http://nginx.org/keys/nginx_signing.key
  pkg.installed:
    - require:
      - pkgrepo: nginx
  service.running:
    - enable: True
    - watch:
      - file: /etc/nginx/conf.d/default.conf

/etc/nginx/conf.d/default.conf:
  file.managed:
    - source: salt://nginx/default.conf

/srv/www:
  file.recurse:
    - source: salt://nginx/www
