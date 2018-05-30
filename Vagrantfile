# -*- mode: ruby -*-
# vi: set ft=ruby :

# Specify minimum Vagrant version and Vagrant API version
Vagrant.require_version '>= 1.6.0'
VAGRANTFILE_API_VERSION = '2'

# Explicitly specify the VMware provider (required for nested virtualization)
ENV['VAGRANT_DEFAULT_PROVIDER'] = 'vmware_fusion'

# Read YAML file with machine details
machines = YAML.load_file(File.join(File.dirname(__FILE__), 'machines.yaml'))

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|

  # Always use Vagrant's default insecure key
  config.ssh.insert_key = false

  # Iterate through entries in YAML file to create VMs
  machines.each do |machine|

    # Configure the VMs per details in machines.yaml
    config.vm.define machine['name'] do |named|

      # Use an Ubuntu 16.04 LTS 64-bit box
      named.vm.box = "jamesoliver/xenial64"

      # Don't check for box updates
      named.vm.box_check_update = false

      # Specify the hostname of the VM
      named.vm.hostname = machine['name']

      # Configure vCPU/RAM per details in machines.yaml
      named.vm.provider 'vmware_fusion' do |provider|
        provider.vmx['memsize'] = machine['ram']
        provider.vmx['numvcpus'] = machine['vcpu']
        provider.vmx['ethernet0.pcislotnumber'] = '33'

        # Enable virtualization extensions (required for KVM)
        provider.vmx['vhv.enable'] = 'TRUE'
      end # named.vm.provider

      # Provision once with Ansible provisioner after all VMs are up and ready
      if machine['name'] == machines[machines.length-1]['name']
        named.vm.provision 'ansible' do |ansible|
          ansible.limit = "all"
          ansible.playbook = "ansible/main.yaml"
        end # named.vm.provision
      end

    end # config.vm.define
  end # machines.each
end # Vagrant.configure
