/srv/www:
	file.recurse:
		- source: salt://nginx/www
		- clean: True

/etc/nginx/nginx.conf:
	file.managed:
		- source: salt://nginx/nginx.conf
		- makedirs: True

/etc/telegraf.d/nginx.conf:
	file.managed:
		- source: salt://nginx/telegraf.conf
		- watch_in:
			- dockerng: telegraf

/etc/nginx/cert.json:
	file.serialize:
		- formatter: json
		- dataset:
				CN: web.local
				key:
					algo: rsa
					size: 2048
				names:
					- C: SE
						L: Stockholm
						O: Load Impact
						OU: k6
	cmd.watch:
		- name: /usr/local/bin/cfssl gencert -ca /etc/ssl/ca/ca.pem -ca-key /etc/ssl/ca/ca-key.pem -hostname=web.local{% for ip in grains.ipv4 %},{{ ip }}{% endfor %} /etc/nginx/cert.json | cfssljson -bare /etc/nginx/cert
		- watch:
			- file: /etc/nginx/cert.json

nginx:
	pkgrepo.managed:
		- name: deb http://nginx.org/packages/ubuntu/ xenial nginx
		- key_url: http://nginx.org/keys/nginx_signing.key
	pkg.installed:
		- require:
			- pkgrepo: nginx
	service.running:
		- enable: True
		- reload: True
		- watch:
			- file: /etc/nginx/nginx.conf
			- cmd: /etc/nginx/cert.json
