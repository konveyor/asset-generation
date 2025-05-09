# asset-generation
Library repository for discovery and transformation of source platforms with Konveyor.

## Code of Conduct
Refer to Konveyor's Code of Conduct [here](https://github.com/konveyor/community/blob/main/CODE_OF_CONDUCT.md).

## Deploy CF on Bosh-lite (on VB)

* install VirtualBox
  `sudo dnf install virtualbox`
* install extension pack `VBoxManage extpack install --replace Oracle_VirtualBox_Extension_Pack-7.1.8.vbox-extpack`
* verify `VBoxManage list extpacks`
* reboot
  
* Follow instruction for bosh lite here https://bosh.io/docs/bosh-lite/
  
  ```bash
  git clone https://github.com/cloudfoundry/bosh-deployment ~/workspace/bosh-deployment
  mkdir -p ~/deployments/vbox
  cd ~/deployments/vbox
  bosh create-env ~/workspace/bosh-deployment/bosh.yml \
    --state ./state.json \
    -o ~/workspace/bosh-deployment/virtualbox/cpi.yml \
    -o ~/workspace/bosh-deployment/virtualbox/outbound-network.yml \
    -o ~/workspace/bosh-deployment/bosh-lite.yml \
    -o ~/workspace/bosh-deployment/bosh-lite-runc.yml \
    -o ~/workspace/bosh-deployment/uaa.yml \
    -o ~/workspace/bosh-deployment/credhub.yml \
    -o ~/workspace/bosh-deployment/jumpbox-user.yml \
    --vars-store ./creds.yml \
    -v director_name=bosh-lite \
    -v internal_ip=192.168.56.6 \
    -v internal_gw=192.168.56.1 \
    -v internal_cidr=192.168.56.0/24 \
    -v outbound_network_name=NatNetwork
  ```
verify vms creation:

```bash
» VBoxManage list vms
"sc-7ff79fac-045c-46e2-437f-ba09d54c40bd" {ca1d6ba0-e95d-4849-9d0f-f7723386951e}
"vm-56718394-d4c1-4b94-6ce3-3d22c712844b" {56718394-d4c1-4b94-6ce3-3d22c712844b}

```

```bash
export BOSH_CLIENT=admin
export BOSH_CLIENT_SECRET=`bosh int ./creds.yml --path /admin_password`
bosh alias-env vbox -e 192.168.56.6 --ca-cert <(bosh int ./creds.yml --path /director_ssl/ca)
```

if it returns somenthing like this, it's ok
```bash
Using environment '192.168.56.6' as client 'admin'

Name               bosh-lite  
UUID               d0207431-94e8-416e-a8ef-73f007d171c4  
Version            282.0.4 (00000000)  
Director Stemcell  -/1.822  
CPI                warden_cpi  
Features           config_server: enabled  
                   local_dns: enabled  
                   snapshots: disabled  
User               admin  

Succeeded
helios02 :: 
```

Optionally, set up a local route for bosh ssh commands or accessing VMs directly
`sudo ip route add 10.244.0.0/16 via 192.168.56.6 # Linux (using iproute2 suite)`

try ping 
`ping 192.168.56.6`

then try 
`bosh -e vbox env`

they should succeed both.


<!-- (Fix the yml path)


the cloud config tells the BOSH Director how to provision VMs, networks, disks, and other IaaS-specific resources in your environment. -->

`bosh -e vbox update-cloud-config ~/cf-deployment/iaas-support/bosh-lite`

`bosh -e vbox upload-stemcell https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-jammy-go_agent`


```bash
git clone https://github.com/cloudfoundry/cf-deployment.git
cd cf-deployment
bosh -e vbox -d cf deploy ~/cf-deployment/cf-cloud-config.yml \
  -o operations/bosh-lite.yml \
  -v system_domain=bosh-lite.com
  -n
```



### Troubleshooting
if you can't create vms, delete old state

```bash 
rm -f ./state.json ./creds.yml
rm -rf ~/.bosh/installations
```
and rerun 
```bash
bosh create-env  [..omissis..]
```
