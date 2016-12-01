variable "access_key" {}
variable "secret_key" {}
variable "region" { default = "eu-west-1" }
variable "ami" { default = "ami-a4d44ed7" }
variable "key_name" { default = "k6-test" }

output "loadgen_ip" {
	value = "${aws_instance.loadgen.public_ip}"
}
output "influx_ip" {
	value = "${aws_instance.influx.public_ip}"
}
output "web_ip" {
	value = "${aws_instance.web.public_ip}"
}

provider "aws" {
	access_key = "${var.access_key}"
	secret_key = "${var.secret_key}"
	region = "${var.region}"
}

resource "aws_security_group" "group" {
	name = "k6-test"
	description = "Security group for k6 test setups"
	
	ingress {
		from_port = 0
		to_port = 0
		protocol = "-1"
		cidr_blocks = ["0.0.0.0/0"]
	}
	
	egress {
		from_port = 0
		to_port = 0
		protocol = "-1"
		cidr_blocks = ["0.0.0.0/0"]
	}
}

resource "aws_placement_group" "group" {
	name = "k6-test"
	strategy = "cluster"
}

resource "aws_instance" "loadgen" {
	instance_type = "m4.xlarge"
	ami = "${var.ami}"
	placement_group = "${aws_placement_group.group.name}"
	security_groups = ["${aws_security_group.group.name}"]
	key_name = "${var.key_name}"
	tags {
		Name = "sbt-loadgen"
	}
	ebs_optimized = true

	connection {
		user = "ubuntu"
		private_key = "${file("${var.key_name}.pem")}"
	}
	provisioner "remote-exec" {
		inline = [
			"mkdir -p /home/ubuntu/go/src/github.com/loadimpact/k6",
			"echo 'export GOPATH=$HOME/go' >> /home/ubuntu/.profile",
			"echo 'export PATH=$PATH:$GOPATH/bin' >> /home/ubuntu/.profile",
			"sudo mkdir -p /etc/salt",
			"sudo ln -s /home/ubuntu/go/src/github.com/loadimpact/k6/external/aws/salt/master.yml /etc/salt/master",
			"sudo ln -s /home/ubuntu/go/src/github.com/loadimpact/k6/external/aws/salt/grains_loadgen.yml /etc/salt/grains",
		]
	}
	provisioner "file" {
		source = "../../"
		destination = "/home/ubuntu/go/src/github.com/loadimpact/k6"
	}
	provisioner "remote-exec" {
		inline = [
			"curl -L https://bootstrap.saltstack.com | sudo sh -s -- -n -M -A 127.0.0.1 -i loadgen stable 2016.3.1",
		]
	}
}

resource "aws_instance" "influx" {
	instance_type = "m4.xlarge"
	ami = "${var.ami}"
	placement_group = "${aws_placement_group.group.name}"
	security_groups = ["${aws_security_group.group.name}"]
	key_name = "${var.key_name}"
	tags {
		Name = "sbt-influx"
	}
	ebs_optimized = true

	connection {
		user = "ubuntu"
		private_key = "${file("${var.key_name}.pem")}"
	}
	provisioner "remote-exec" {
		inline = [
			"sudo mkdir -p /etc/salt",
			"sudo touch /etc/salt/grains",
			"sudo chown ubuntu:ubuntu /etc/salt/grains",
		]
	}
	provisioner "file" {
		source = "salt/grains_influx.yml"
		destination = "/etc/salt/grains"
	}
	provisioner "remote-exec" {
		inline = [
			"curl -L https://bootstrap.saltstack.com | sudo sh -s -- -n -A ${aws_instance.loadgen.private_ip} -i influx stable 2016.3.1"
		]
	}
}

resource "aws_instance" "web" {
	instance_type = "m4.xlarge"
	ami = "${var.ami}"
	placement_group = "${aws_placement_group.group.name}"
	security_groups = ["${aws_security_group.group.name}"]
	key_name = "${var.key_name}"
	tags {
		Name = "sbt-web"
	}
	ebs_optimized = true

	connection {
		user = "ubuntu"
		private_key = "${file("${var.key_name}.pem")}"
	}
	provisioner "remote-exec" {
		inline = [
			"sudo mkdir -p /etc/salt",
			"sudo touch /etc/salt/grains",
			"sudo chown ubuntu:ubuntu /etc/salt/grains",
		]
	}
	provisioner "file" {
		source = "salt/grains_web.yml"
		destination = "/etc/salt/grains"
	}
	provisioner "remote-exec" {
		inline = [
			"curl -L https://bootstrap.saltstack.com | sudo sh -s -- -n -A ${aws_instance.loadgen.private_ip} -i web stable 2016.3.1"
		]
	}
}
