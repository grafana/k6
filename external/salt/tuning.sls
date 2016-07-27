/etc/security/limits.d/nofile.conf:
  file.managed:
    - contents:
      - '* soft nofile 1048576'
      - '* hard nofile 1048576'

vm.swappiness:
  sysctl.present:
    - value: 0

net.ipv4.ip_local_port_range:
  sysctl.present:
    - value: 100 65535

net.ipv4.tcp_tw_reuse:
  sysctl.present:
    - value: 1

net.ipv4.tcp_max_tw_buckets:
  sysctl.present:
    - value: 6000000

net.ipv4.tcp_max_syn_backlog:
  sysctl.present:
    - value: 65535

net.ipv4.tcp_max_orphans:
  sysctl.present:
    - value: 65536

net.ipv4.tcp_fin_timeout:
  sysctl.present:
    - value: 10

net.core.somaxconn:
  sysctl.present:
    - value: 16384

net.core.netdev_max_backlog:
  sysctl.present:
    - value: 16384

net.ipv4.tcp_synack_retries:
  sysctl.present:
    - value: 2

net.ipv4.tcp_syn_retries:
  sysctl.present:
    - value: 2
