# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure(2) do |config|
	config.vm.box = "boxcutter/ubuntu1604"
	config.vm.synced_folder ".", "/vagrant", disabled: true

	config.vm.define "loadgen", primary: true do |loadgen|
		loadgen.vm.provider "virtualbox" do |vb|
			vb.memory = 2048
			vb.cpus = 2
		end
		
		loadgen.vm.hostname = "loadgen"
		loadgen.vm.network "private_network", ip: "172.16.0.2"

		loadgen.vm.synced_folder "external/salt", "/srv/salt", type: "rsync"
		loadgen.vm.synced_folder "external/pillar", "/srv/pillar", type: "rsync"
		loadgen.vm.synced_folder ".", "/home/vagrant/go/src/github.com/loadimpact/k6", type: "rsync"

		loadgen.vm.provision :salt do |salt|
			salt.bootstrap_options = "-F -c /tmp -i loadgen"
			salt.grains_config = "external/vagrant/loadgen_grains.yml"
			salt.minion_config = "external/vagrant/salt_minion.yml"
			salt.minion_key = "external/vagrant/loadgen.pem"
			salt.minion_pub = "external/vagrant/loadgen.pub"
			salt.install_master = true
			salt.master_config = "external/vagrant/salt_master.yml"
			salt.seed_master = {
				loadgen: "external/vagrant/loadgen.pub",
				influx:  "external/vagrant/influx.pub",
				web:     "external/vagrant/web.pub",
			}
		end
	end

	config.vm.define "influx" do |influx|
		influx.vm.provider "virtualbox" do |vb|
			vb.memory = 2048
			vb.cpus = 2
		end
		
		influx.vm.hostname = "influx"
		influx.vm.network "private_network", ip: "172.16.0.3"

		influx.vm.provision :salt do |salt|
			salt.bootstrap_options = "-F -c /tmp -i influx"
			salt.grains_config = "external/vagrant/influx_grains.yml"
			salt.minion_config = "external/vagrant/salt_minion.yml"
			salt.minion_key = "external/vagrant/influx.pem"
			salt.minion_pub = "external/vagrant/influx.pub"
		end
	end

	config.vm.define "web" do |web|
		web.vm.provider "virtualbox" do |vb|
			vb.memory = 2048
			vb.cpus = 2
		end
		
		web.vm.hostname = "web"
		web.vm.network "private_network", ip: "172.16.0.4"

		web.vm.provision :salt do |salt|
			salt.bootstrap_options = "-F -c /tmp -i web"
			salt.grains_config = "external/vagrant/web_grains.yml"
			salt.minion_config = "external/vagrant/salt_minion.yml"
			salt.minion_key = "external/vagrant/web.pem"
			salt.minion_pub = "external/vagrant/web.pub"
		end
	end
end
