# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure(2) do |config|

  if (/cygwin|mswin|mingw|bccwin|wince|emx/ =~ RUBY_PLATFORM) != nil
    config.vm.synced_folder ".", "/maasdev", mount_options: ["dmode=700,fmode=600"]
  else
    config.vm.synced_folder ".", "/maasdev"
  end

  config.vm.define "maasdev" do |d|
    d.vm.box = "ubuntu/trusty64"
    d.vm.hostname = "maasdev"
    d.vm.network "private_network", ip: "10.100.198.200"
    d.vm.provision :shell, path: "scripts/bootstrap_ansible.sh"
    d.vm.provision :shell, inline: "PYTHONUNBUFFERED=1 ansible-playbook /maasdev/ansible/maasdev.yml -c local"
    d.vm.provider "virtualbox" do |v|
      v.memory = 2048
    end
  end

  config.vm.define "prod" do |d|
    d.vm.box = "ubuntu/trusty64"
    d.vm.hostname = "prod"
    d.vm.network "private_network", ip: "10.100.198.201"
    d.vm.provider "virtualbox" do |v|
      v.memory = 1024
    end
  end

  config.vm.define "headnode" do |h|
    h.vm.box = "ubuntu/trusty64"
    h.vm.hostname = "headnode"
    h.vm.network "private_network",
      ip: "10.100.198.202"
    h.vm.network "private_network",
      ip: "10.6.0.1",
      virtualbox__intnet: "cord-test-network"
    h.vm.provider "virtualbox" do |v|
      v.memory = 2048
    end
  end

  config.vm.define "computenode" do |c|
    #c.vm.box = "ubuntu/trusty64"
    c.vm.box = "clink15/pxe"
    c.vm.synced_folder '.', '/vagrant', disable: true
    c.vm.communicator = "none"
    c.vm.hostname = "computenode"
    c.vm.network "private_network",
      adapter: "1",
      type: "dhcp",
      auto_config: false,
      virtualbox__intnet: "cord-test-network"
    c.vm.provider "virtualbox" do |v|
      v.memory = 1048
      v.gui = "true"
    end
  end

  if Vagrant.has_plugin?("vagrant-cachier")
    config.cache.scope = :box
  end
  
end
