golang:
  pkgrepo.managed:
    - ppa: ubuntu-lxc/lxd-stable
  pkg.latest:
    - require:
      - pkgrepo: golang
