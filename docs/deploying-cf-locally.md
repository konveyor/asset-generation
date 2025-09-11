# How to deploy Cloud Foundry in Bosh-Lite using cf-deployment
Deploying Cloud Foundry with Bosh-Lite is a low-cost, lightweight approach tailored for development and testing environments. Unlike AWS deployments, Bosh-Lite runs the entire Cloud Foundry stack inside a single VM on your local machine using BOSH’s local CPI (warden). This drastically reduces infrastructure requirements and speeds up prototyping. Use this setup when you need an isolated, disposable CF environment for debugging, experimentation, or learning.

> 💡Note: This environment is not production-grade.<br/>
> ⚠️ Important: This setup is known to work on Fedora 41 and 42.<br/>
> ⚠️ Important: Installing on a VM only works if Nested Virtualization is available.<br/>
> ⚠️ Important: Secure Boot must be disabled, else the VirtualBox modules must be signed with an enrolled MOK (machine owner key), to load.

## Install VirtualBox and prerequisites
Install Fedora 42

### Disable KVM virtualization
VirtualBox cannot be used at the same time as KVM. In recent kernels kvm modules load automatically which can prevent Virtualbox VMs from starting even if KVM is not being actively used and you should choose an approach to prevent conflicts. https://www.virtualbox.org/ticket/22248 contains more information and links to additional resources.

To temporarily disable KVM virtualization run these commands. This will last until you reboot.

```bash
if lsmod | grep -q kvm_intel; then sudo rmmod kvm_intel; fi
if lsmod | grep -q kvm_amd; then sudo rmmod kvm_amd; fi
if lsmod | grep -q kvm; then sudo rmmod kvm; fi
```

To permanently disable KVM virtualization blacklist the modules. This will take effect on reboot and last until the file is removed.

```bash
sudo bash -c 'cat << EOF > /etc/modprobe.d/kvm-blacklist.conf
blacklist kvm
blacklist kvm_amd
blacklist kvm_intel
EOF'
```

The VirtualBox ticket above also suggests adding `kvm.enable_virt_at_load=0` to the kernel commandline. This should also take effect on reboot and prevent kvm modules from loading automatically while allowing them to load if KVM is needed. Although this is more convenient, it appears that in some cases the modules may be loaded by some other process. If you would like to try this approach it can be done with grubby.

```bash
sudo grubby --update-kernel=ALL --args="kvm.enable_virt_at_load=0"
```

And it can be removed with

```bash
sudo grubby --update-kernel=ALL --remove-args="kvm.enable_virt_at_load=0"
```

### Update and Install VirtualBox
```bash
sudo dnf -y update
sudo dnf -y install https://mirrors.rpmfusion.org/free/fedora/rpmfusion-free-release-$(rpm -E %fedora).noarch.rpm https://mirrors.rpmfusion.org/nonfree/fedora/rpmfusion-nonfree-release-$(rpm -E %fedora).noarch.rpm
sudo wget -O /etc/yum.repos.d/cloudfoundry-cli.repo https://packages.cloudfoundry.org/fedora/cloudfoundry-cli.repo
sudo dnf -y install cf8-cli git jq ruby wget yq VirtualBox
sudo bash -c 'cat << EOF >> /etc/hosts
10.244.0.34 bosh-lite.com
10.244.0.34 api.bosh-lite.com
10.244.0.34 log-cache.bosh-lite.com
10.244.0.34 login.bosh-lite.com
10.244.0.34 uaa.bosh-lite.com
EOF'
```

Reboot your system to finish the installation, including building modules for the updated kernel, and module blacklists taking effect. 

```bash
sudo reboot
```

## Install BOSH
Follow instruction in `Bosh-Lite` [website](https://bosh.io/docs/bosh-lite/), for example:

```bash
wget https://github.com/cloudfoundry/bosh-cli/releases/download/v7.9.8/bosh-cli-7.9.8-linux-amd64
chmod +x ./bosh-cli-7.9.8-linux-amd64
mkdir -p ~/.local/bin
mv ./bosh-cli-7.9.8-linux-amd64 ~/.local/bin/bosh
```

## Deploy the BOSH Director with bosh-lite 

```bash
git clone https://github.com/cloudfoundry/bosh-deployment ~/workspace/bosh-deployment
mkdir -p ~/deployments/vbox
cd ~/deployments/vbox
```

By default a VM with 4 CPU and 6GB of RAM will be deployed. This can be adjusted if desired, for example:

```bash
yq e '.[2].value.cpus=16' -i ~/workspace/bosh-deployment/virtualbox/cpi.yml
yq e '.[2].value.memory=16384' -i ~/workspace/bosh-deployment/virtualbox/cpi.yml
```

Create the BOSH Director VM:

```bash
for i in {1..5}; do
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
  -v outbound_network_name=NatNetwork \
  && break
  echo bosh failed, retrying.
done
```
Note: bosh sometimes fails to complete properly. On investigation it appears the NatNetwork interface on eth1 is sometimes unable to obtain an IP address.
Until we can determine why this is the case it is being run here in a bash until loop so that we can retry automatically until we have success. A second attempt frequently succeeds. Perhaps there is a race condition between the NatNetwork being ready and VM creation. TODO: Try creating the NatNetwork in advance.

Check that the VMs have been created in VirtualBox:

```bash
VBoxManage list vms
```

You should see entries similar to:

```bash
"sc-7ff79fac-045c-46e2-437f-ba09d54c40bd" {ca1d6ba0-e95d-4849-9d0f-f7723386951e}
"vm-56718394-d4c1-4b94-6ce3-3d22c712844b" {56718394-d4c1-4b94-6ce3-3d22c712844b}
```

## Configure environment variables and login

```bash
export BOSH_CLIENT=admin
export BOSH_CLIENT_SECRET=`bosh int ./creds.yml --path /admin_password`
bosh alias-env vbox -e 192.168.56.6 --ca-cert <(bosh int ./creds.yml --path /director_ssl/ca)
```

Expected output:

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
```

## Set up routing for VM access
Optionally, set up a local route for bosh ssh commands or accessing VMs
directly:

`sudo ip route add 10.244.0.0/16 via 192.168.56.6 # Linux (using iproute2 suite)`

try `ping -c3 192.168.56.6`

Expected output: 

```bash
PING 192.168.56.6 (192.168.56.6) 56(84) bytes of data.
64 bytes from 192.168.56.6: icmp_seq=1 ttl=64 time=0.364 ms
64 bytes from 192.168.56.6: icmp_seq=2 ttl=64 time=0.372 ms
64 bytes from 192.168.56.6: icmp_seq=3 ttl=64 time=0.301 ms
```

then try `bosh -e vbox env`

Expected output:

```bash
Using environment '192.168.56.6' as client 'admin'

Name               bosh-lite  
UUID               6afb466e-b6ab-4b52-a4d2-c833ae02776a  
Version            282.0.4 (00000000)  
Director Stemcell  -/1.822  
CPI                warden_cpi  
Features           config_server: enabled  
                   local_dns: enabled  
                   snapshots: disabled  
User               admin  

Succeeded
```

## Enable BOSH DNS

```bash
bosh -e vbox update-runtime-config ~/workspace/bosh-deployment/runtime-configs/dns.yml --name dns
```

## Clone the cf-deployment repository

```bash
git clone https://github.com/cloudfoundry/cf-deployment.git ~/cf-deployment
cd ~/cf-deployment
```

To ensure you're using the latest precompiled stemcell version, first check which version is referenced in the `operations/use-compiled-releases.yml` file:

```bash
export STEMCELL_VERSION=$(grep -A 2 stemcell operations/use-compiled-releases.yml | grep version | sort -u | awk -F'"' '{print $2}')
STEMCELL_SHA1=$(curl -s "https://bosh.io/api/v1/stemcells/bosh-warden-boshlite-ubuntu-jammy-go_agent" \
  | jq -r --arg version "$STEMCELL_VERSION" '.[] | select(.version == $version) | .regular.sha1')

echo "Stemcell version: $STEMCELL_VERSION"
echo "SHA1: $STEMCELL_SHA1"
```

Example output:

```bash
Stemcell version: 1.423
SHA1: 4ad3b7265af38de84d83887bf334193259a59981
```

Use the version shown in the output to update your cf-deployment.yml file:

```bash
yq e '.stemcells[0].alias = "default" | .stemcells[0].os = "ubuntu-jammy" | .stemcells[0].version = env(STEMCELL_VERSION)' -i cf-deployment.yml
```

## Upload cloud config and stemcell
The **cloud config** tells the BOSH Director how to provision VMs, networks, disks, and other IaaS-specific resources in your environment.

```bash
bosh -e vbox update-cloud-config ~/cf-deployment/iaas-support/bosh-lite/cloud-config.yml
```

Next, upload the stemcell.

```bash
bosh -e vbox upload-stemcell \
  --sha1 "$STEMCELL_SHA1" \
  "https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-jammy-go_agent?v=${STEMCELL_VERSION}"
```

## Deploy Cloud Foundry

```bash
bosh -n -e vbox -d cf deploy \
  cf-deployment.yml \
  -o operations/bosh-lite.yml \
  -o operations/use-compiled-releases.yml \
  -v system_domain=bosh-lite.com \
  -v stemcell_os=ubuntu-jammy \
  -v stemcell_version=${STEMCELL_VERSION}
```

This process takes around 60-120 minutes.

Make sure to match the `stemcell_version` with the one you uploaded earlier.

> 💡 Note: The system_domain value `bosh-lite.com` is predefined when deploying the
BOSH Director for BOSH Lite.

## Target Cloud Foundry API

Point the CF CLI to your local API:

```bash
cf api https://api.bosh-lite.com --skip-ssl-validation
```

Expected output:

```bash
Setting API endpoint to https://api.bosh-lite.com...
OK

API endpoint:   https://api.bosh-lite.com
API version:    3.193.0

Not logged in. Use 'cf login' or 'cf login --sso' to log in.
```

## Login to Cloud Foundry

### Retrieve credential from CredHub
#### 1. Retrieve Credentials from CredHub

1. **Install CredHub CLI**  
    Follow the [official instructions](https://github.com/cloudfoundry/credhub-cli#installing-the-cli), for example:

```bash
wget https://github.com/cloudfoundry/credhub-cli/releases/download/2.9.48/credhub-linux-amd64-2.9.48.tgz
tar zxvf credhub-linux-amd64-2.9.48.tgz
mkdir -p ~/.local/bin
mv credhub ~/.local/bin
```

1. **Set Up the Environment**

    ```bash
    # Set CredHub server URL and credentials
    export CREDHUB_SERVER=https://192.168.56.6:8844
    export CREDHUB_CLIENT=credhub-admin
    export CREDHUB_SECRET=$(bosh int ~/deployments/vbox/creds.yml --path /credhub_admin_client_secret)

    # Extract the CA certificate
    bosh int ~/deployments/vbox/creds.yml --path /credhub_tls/ca > credhub-ca.crt
    export CREDHUB_CA_CERT=./credhub-ca.crt
    ```

1. **Initialize CredHub CLI**

    ```bash
    credhub api $CREDHUB_SERVER --ca-cert=$CREDHUB_CA_CERT --skip-tls-validation
    ```

    Expected output:

    ```bash
    Setting the target url: https://192.168.56.6:8844
    ```

    See troubleshooting section [Can't connect to the auth server via
    credhub](#cant-connect-to-the-auth-server-via-credhub) in case of issues.

1. **Verify CredHub Access**

    Verify that `credhub` is working:

    ```bash
    credhub find
    ```

    Expected output:

    ```bash
    credentials:
        - name: /dns_api_client_tls
        version_created_at: "2025-05-14T11:25:49Z"
        - name: /dns_api_server_tls
        version_created_at: "2025-05-14T11:25:49Z"
        - name: /dns_api_tls_ca
        version_created_at: "2025-05-14T11:25:49Z"
        - name: /dns_healthcheck_client_tls
        version_created_at: "2025-05-14T11:25:48Z"
        - name: /dns_healthcheck_server_tls
        ... etc ...
    ```

1. **Retrieve CF Admin Password and Log In**
    Set CF API endpoint

    ```bash
    cf api https://api.bosh-lite.com --skip-ssl-validation
    ```
    
    Expected output:

    ```
    cf api https://api.bosh-lite.com --skip-ssl-validation
    Setting API endpoint to https://api.bosh-lite.com...
    OK

    API endpoint:   https://api.bosh-lite.com
    API version:    3.193.0

    Not logged in. Use 'cf login' or 'cf login --sso' to log in.
    ```

    Get the admin password from CredHub and login

    ```bash
    CF_ADMIN_PASSWORD=$(credhub get -n /bosh-lite/cf/cf_admin_password -q)
    cf login -a https://api.bosh-lite.com --skip-ssl-validation -u admin -p "$CF_ADMIN_PASSWORD"
    ```

    If successful, the expected output is:

    ```bash
    API endpoint: https://api.bosh-lite.com

    Authenticating...
    OK

    Targeted org system.

    API endpoint:   https://api.bosh-lite.com
    API version:    3.193.0
    user:           admin
    org:            system
    space:          No space targeted, use 'cf target -s SPACE'
    ```

    ✅ You are now logged in and ready to use CF.

## Deploy an Example App

Create an organization and a space, target them, and push an example Docker-based app:

```bash
cf create-org org && cf create-space -o org space && cf target -o org -s space
```

Check current feature flags:
`cf feature-flags`

Expected output

```bash
Getting feature flags as admin...

name                                          state
app_bits_upload                               enabled
app_scaling                                   enabled
diego_cnb                                     disabled
diego_docker                                  disabled
```

Look for the `diego_docker` flag — it will likely show `disabled`.

Enable Docker support:

```bash
cf enable-feature-flag diego_docker
```

Push the example application:

```
cf push nginx --docker-image nginxinc/nginx-unprivileged:1.23.2
```

Add a DNS entry for the application:

```bash
sudo bash -c 'cat << EOF >> /etc/hosts
10.244.0.34 nginx.bosh-lite.com
EOF'

Once deployed, test the app using `curl`:

```bash
curl http://nginx.bosh-lite.com
```

Expected output:

```html
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

# Connect to a remote Cloud Foundry instance

**Option A: SSH Tunnel**

1. Update `/etc/hosts` on Your Local Machine

    Add the following lines to your `/etc/hosts` file:

    ```bash
    127.0.0.1 api.bosh-lite.com
    127.0.0.1 login.bosh-lite.com
    127.0.0.1 uaa.bosh-lite.com
    ```

2. Set up ssh tunnel
   * Share your _**public**_ ssh key with the remote system admin.
   * Once access is granted, verify your SSH connection:

      ```bash
      ssh <user_remote>@<remote_server_address> -i <path_to/private/sshkey>
      ```
      > Note: Use the path to your private SSH key, not the public key.

   * Set up the SSH tunnel on your local machine:

      ```bash
      sudo ssh -v -N \
        -i <path_to/private/sshkey> \
        -L 443:10.244.0.131:443 \
        -L 8443:10.244.0.34:443 \
        -L 8444:10.244.0.131:443 \
        <user_remote>@<remote_server_address>
      ```

    > Extra info:<br/>
    > The `-N` flag tells SSH not to execute a remote command.<br/>
    > The `-v` flag enables verbose output for debugging.

**Option B: iptables rules (persistent across reboots)**

As an alternative to SSH tunneling, you can use iptables rules for persistent port forwarding:

1. Login to the VM that has CF deployed:

    ```bash
    ssh <user_remote>@<remote_server_address> -i <path_to/private/sshkey>
    ```

2. Set up the NAT rules to redirect traffic:

    ```bash
    sudo iptables -t nat -A PREROUTING -p tcp --dport 443 -j DNAT --to-destination 10.244.0.131:443
    sudo iptables -t nat -A PREROUTING -p tcp --dport 8443 -j DNAT --to-destination 10.244.0.34:443
    sudo iptables -t nat -A PREROUTING -p tcp --dport 8444 -j DNAT --to-destination 10.244.0.131:443
    sudo iptables -t nat -A POSTROUTING -d 10.244.0.0/16 -j MASQUERADE
    
    # Enable forwarded traffic via filter table
    sudo iptables -A FORWARD -p tcp -d 10.244.0.131 --dport 443 \
      -m state --state NEW,ESTABLISHED,RELATED -j ACCEPT
    sudo iptables -A FORWARD \
      -m state --state ESTABLISHED,RELATED -j ACCEPT
    ```

3. Enable IP forwarding in the kernel:

    ```bash
    sudo sysctl -w net.ipv4.ip_forward=1
    ```

4. Make IP forwarding persistent across reboots:

    ```bash
    echo "net.ipv4.ip_forward = 1" | sudo tee -a /etc/sysctl.conf
    sudo sysctl -p
    ```

    > Note: Only the sysctl change is persistent here. iptables rules are NOT persistent by default.
    > To persist them, choose one:
    > * firewalld (recommended on Fedora/nftables): translate rules to `firewall-cmd` (e.g., `--add-forward-port`, `--add-masquerade`) with `--permanent`, then `firewall-cmd --reload`.
    > * iptables-services (replaces firewalld): `sudo dnf install -y iptables-services && sudo iptables-save | sudo tee /etc/sysconfig/iptables && sudo systemctl enable --now iptables`
    > * systemd unit: apply your iptables commands on boot.

5. Update your local machine /etc/hosts file: 
    
    Add the following lines to your local machine's `/etc/hosts` file (replace `<VM_IP>` with the actual IP of the VM that has CF installed):

    ```bash
    <VM_IP> api.bosh-lite.com
    <VM_IP> login.bosh-lite.com
    <VM_IP> uaa.bosh-lite.com
    ```

6. Login to CF: 

    ```bash
    # If you have credentials:
    cf login -a https://api.bosh-lite.com --skip-ssl-validation -u admin -p "<ADMIN_PASSWORD>"
    # Alternatively, expose CredHub (port 8844) and retrieve the password as shown in the earlier section.
    cf login -a https://api.bosh-lite.com --skip-ssl-validation -u admin -p "$CF_ADMIN_PASSWORD"
    ```

7. Verify Access to the Remote Cloud Foundry Instance
   Open a new terminal on your local machine and check access to the remote CF instance

    ```bash
    ➜ cf apps
    Getting apps in org org / space space as admin...

    name    requested state   processes   routes
    nginx   started           web:1/1     nginx.bosh-lite.com
    ```

# Reboot Process
Normally the development environment does not survive through rebooting the host, however it is possible to preserve the environment so it is not necessary to redeploy.

To prepare install xmlstarlet:

```bash
sudo dnf install xmlstarlet
```

Before rebooting:

```bash
export VMUUID=$(VBoxManage list vms | grep vm- | awk -F '[{}]' '{ print $2 }' )
VBoxManage controlvm vm-$VMUUID savestate
```

After rebooting:

```bash
export VMUUID=$(VBoxManage list vms | grep vm- | awk -F '[{}]' '{ print $2 }' )
xmlstarlet edit --inplace -N s=http://www.virtualbox.org/ \
  -u "s:VirtualBox/s:Machine/s:Hardware/s:StorageControllers/s:StorageController[@type='AHCI']/s:AttachedDevice[@port=2]/@hotpluggable" \
  -v "true" VirtualBox\ VMs/vm-$VMUUID/vm-$VMUUID.vbox
VBoxManage startvm vm-$VMUUID --type headless
sudo ip route add 10.244.0.0/16 via 192.168.56.6
```

# Troubleshooting
## ❌ Can't create VMs?

  Remove old BOSH state and credentials:

  ```bash
  rm -f ./state.json ./creds.yml
  rm -rf ~/.bosh/installations
  ```

  Then rerun:

  ```bash
  bosh create-env  [..omissis..]
  ```

## ❌ Can't use `credhub`?

Make sure you can reach the `CredHub` VM via SSH:
`bosh -e vbox -d cf ssh credhub`
