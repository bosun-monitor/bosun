VAGRANTFILE_API_VERSION = "2"  
Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|

  config.vm.define "tsdb" do |v|
    v.vm.provider "docker" do |d|
      d.image = "petergrace/opentsdb-docker"
      d.name = "tsdb"
      d.vagrant_vagrantfile = "./Vagrantfile.proxy"
    end
  end

  config.vm.define "bosun" do |v|
    v.vm.provider "docker" do |d|
      d.build_dir = "."
      d.ports = ["8070:8070", "4242:4242"]
      d.link("tsdb:tsdb")
      d.vagrant_vagrantfile = "./Vagrantfile.proxy"
    end
  end

end