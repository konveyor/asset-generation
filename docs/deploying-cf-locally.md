# How to deploy Cloud Foundry in Bosh-Lite using cf-deployment
Deploying Cloud Foundry with Bosh-Lite is a low-cost, lightweight approach tailored for development and testing environments. Unlike AWS deployments, Bosh-Lite runs the entire Cloud Foundry stack inside a single VM on your local machine using BOSH’s local CPI (warden). This drastically reduces infrastructure requirements and speeds up prototyping. Use this setup when you need an isolated, disposable CF environment for debugging, experimentation, or learning.

> 💡 Note: This environment is not production-grade.

> ⚠️ Important: This setup only works on bare-metal Fedora 41.<br/>
> Installing it on a VM or a virtualized host does not work and is not supported.

## Install VirtualBox and prerequisites
 
Ensure that VirtualBox and its extension pack are installed and configured
properly.

> 💡 **Note:** Always download and install the latest version of the VirtualBox Extension Pack that matches your installed VirtualBox version. You can find it on the [official VirtualBox downloads page](https://www.virtualbox.org/wiki/Downloads).

```bash
sudo dnf install virtualbox
# Replace with the latest version of the Extension Pack
VBoxManage extpack install --replace Oracle_VirtualBox_Extension_Pack-7.1.8.vbox-extpack
```
After installation, verify that the extension pack was installed correctly:

```bash
VBoxManage list extpacks
```
You should see output similar to the following:

```bash
Extension Packs: 1
Pack no. 0:   Oracle VirtualBox Extension Pack
Version:        7.1.8
Revision:       168469
Edition:        
Description:    Oracle Cloud Infrastructure integration, Host Webcam, VirtualBox RDP, PXE ROM, Disk Encryption, NVMe, full VM encryption.
VRDE Module:    VBoxVRDP
Crypto Module:  VBoxPuelCrypto
Usable:         true
Why unusable:
```

Make sure `Usable: true` is present in the output — this indicates that the extension pack is functioning correctly.
If `Why unusable:` contains any message, the extension pack installation may
have issues (e.g., version mismatch or missing dependencies).

Reboot your system after installation:

```bash
sudo reboot
```

## Deploy the BOSH Director with bosh-lite 
Follow instruction in `Bosh-Lite` [website](https://bosh.io/docs/bosh-lite/)

```bash
git clone https://github.com/cloudfoundry/bosh-deployment ~/workspace/bosh-deployment
mkdir -p ~/deployments/vbox
cd ~/deployments/vbox
```
Create the BOSH Director VM:

```bash
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
helios02 :: 
```
## Set up routing for VM access
Optionally, set up a local route for bosh ssh commands or accessing VMs
directly:

`sudo ip route add 10.244.0.0/16 via 192.168.56.6 # Linux (using iproute2 suite)`

try `ping 192.168.56.6`

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


## Clone the cf-deployment repository

```bash
git clone https://github.com/cloudfoundry/cf-deployment.git
cd cf-deployment
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

Use the version shown in the output to update your cf-deployment.yaml file:
```bash
yq e '.stemcells[0].alias = "default" | .stemcells[0].os = "ubuntu-jammy" | .stemcells[0].version = env(STEMCELL_VERSION)' -i cf-deployment.yaml
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
This process takes around 30–60 minutes.

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
    Follow the [official instructions](https://github.com/cloudfoundry/credhub-cli#installing-the-cli)

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
cf create-org org && cf create-space -o org space && cf target -o org
cf push nginx --docker-image nginxinc/nginx-unprivileged:1.23.2
```
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

## ❌ Can't Push Docker Images?
If you're seeing an error like this when pushing a Docker image:

```bash
cf push nginx --docker-image nginxinc/nginx-unprivileged:1.23.2
Pushing app nginx to org org / space space as admin...
For application 'nginx': Feature Disabled: diego_docker
FAILED
```

This means Docker support is not enabled in your Cloud Foundry deployment. By default, CF disables the [diego_docker feature flag](https://docs.cloudfoundry.org/adminguide/docker.html), which is required to push and run Docker images on Diego.
Check the current feature flags:
`cf feature-flags`

Look for the `diego_docker` flag — it will likely show `disabled`.

Enable Docker support:
```bash
cf enable-feature-flag diego_docker
```
After enabling the flag, retry your `cf push` command.
