mine_functions:
  private_ips:
    mine_function: network.ip_addrs
    interface: {{ 'enp0s8' if grains.get('vagrant', False) else 'ens3' }}
