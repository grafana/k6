docker:
  group.present:
    - system: True
    {% if grains.get('vagrant', False) %}
    - addusers:
      - vagrant
    {% endif %}
  pkgrepo.managed:
    - name: deb https://apt.dockerproject.org/repo ubuntu-xenial main
    - keyserver: hkp://p80.pool.sks-keyservers.net:80
    - keyid: 58118E89F3A912897C070ADBF76221572C52609D
    - require:
      - pkg: apt-transport-https
  pkg.installed:
    - name: docker-engine
    - require:
      - pkgrepo: docker
      - group: docker
  service.running:
    - enable: True
    - require:
      - pkg: docker

docker-py:
  pip.installed
