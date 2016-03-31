# -*- mode: ruby -*-
# vi: set ft=ruby :

# Helper for boilerplate involved with setting resource limits across providers
def set_limits(box, cpus: 1, memory: 512)
  box.vm.provider "virtualbox" do |vb|
    vb.memory = memory
    vb.cpus = cpus
  end
  box.vm.provider "vmware_fusion" do |vw|
    vw.vmx["memsize"] = memory
    vw.vmx["numvcpus"] = cpus
  end
end

# Documentation: https://docs.vagrantup.com.
Vagrant.configure(2) do |config|
  config.vm.box = "boxcutter/debian82"
  
  # Default machine, used to run the load generator
  config.vm.define "default", primary: true do |default|
    set_limits default, cpus: 2, memory: 512
    default.vm.network "private_network", ip: "10.20.30.10"
    
    default.vm.provision "salt" do |salt|
      # Colorize output properly, overriding Vagrant's default
      salt.colorize = true
      # Required for Vagrant
      salt.bootstrap_options = "-F -c /tmp -i default"
      # Configure the salt minion
      salt.minion_config = "_provisioning/vagrant/default/salt_minion.yml"
      salt.minion_pub = "_provisioning/vagrant/default/default.pub"
      salt.minion_key = "_provisioning/vagrant/default/default.pem"
      # Install the salt-master
      salt.install_master = true
      salt.master_config = "_provisioning/vagrant/default/salt_master.yml"
      # Preseed our own key, this master is set to naively auto-accept
      salt.seed_master = { default: salt.minion_pub }
      # Run a highstate when provisioning
      salt.run_highstate = true
    end
  end
  
  # Target machine, hosts servers that can be tested
  config.vm.define "target" do |target|
    set_limits target, cpus: 2, memory: 512
    target.vm.network "private_network", ip: "10.20.30.20"
    
    target.vm.provision "salt" do |salt|
      salt.colorize = true
      salt.bootstrap_options = "-F -c /tmp -i target"
      salt.minion_config = "_provisioning/vagrant/target/salt_minion.yml"
      salt.minion_pub = "_provisioning/vagrant/target/target.pub"
      salt.minion_key = "_provisioning/vagrant/target/target.pem"
      salt.run_highstate = true
    end
  end
end
