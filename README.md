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

if it returns something like this, it's ok
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


<!--
the cloud config tells the BOSH Director how to provision VMs, networks, disks, and other IaaS-specific resources in your environment. -->

`bosh -e vbox update-cloud-config ~/cf-deployment/iaas-support/bosh-lite/cloud-config.yml`

```bash
bosh -e vbox upload-stemcell \
  --sha1 4ad3b7265af38de84d83887bf334193259a59981 \
  "https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-jammy-go_agent?v=1.423"
```


```bash
git clone https://github.com/cloudfoundry/cf-deployment.git
cd cf-deployment
#Update default value to use precompiled release
yq e '.stemcells[0].alias = "default" | .stemcells[0].os = "ubuntu-jammy" | .stemcells[0].version = "1.423"' -i cf-deployment.yaml
bosh -n -e vbox -d cf deploy \
  cf-deployment.yml \
  -o operations/bosh-lite.yml \
  -o operations/use-compiled-releases.yml \
  -v system_domain=bosh-lite.com \
  -v stemcell_os=ubuntu-jammy \
  -v stemcell_version=1.423
```

Verify that you can set target api

```bash
» cf api https://api.bosh-lite.com --skip-ssl-validation
Setting API endpoint to https://api.bosh-lite.com...
OK

API endpoint:   https://api.bosh-lite.com
API version:    3.193.0

Not logged in. Use 'cf login' or 'cf login --sso' to log in.
```


# Login to CF

## Retrieve credential from CredHub
# Login to Cloud Foundry (CF)

## 1. Retrieve Credentials from CredHub

### Step 1: Install CredHub CLI

Follow the instructions 👉 [here](https://github.com/cloudfoundry/credhub-cli#installing-the-cli)

### Step 2: Set Up the Environment

```bash
# Set CredHub server URL and credentials
export CREDHUB_SERVER=https://192.168.56.6:8844
export CREDHUB_CLIENT=credhub-admin
export CREDHUB_SECRET=$(bosh int ~/deployments/vbox/creds.yml --path /credhub_admin_client_secret)

# Extract the CA certificate
bosh int ~/deployments/vbox/creds.yml --path /credhub_tls/ca > credhub-ca.crt
export CREDHUB_CA_CERT=./credhub-ca.crt
```

Initialize CredHub CLI running:
```bash
credhub api $CREDHUB_SERVER --ca-cert=$CREDHUB_CA_CERT
```

the expected output is:

```bash
Setting the target url: https://192.168.56.6:8844
```

see troubleshooting section [Can't connect to the auth server via credhub](#cant-connect-to-the-auth-server-via-credhub)

### Step 3: Verify CredHub Access

Run the following command to verify that `credhub` is working:

```bash
credhub find
```

Expected output example:
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
### Step 4: Retrieve CF Admin Password and Log In
```bash
# Get the admin password from CredHub
CF_ADMIN_PASSWORD=$(credhub get -n /bosh-lite/cf/cf_admin_password --output-json | jq -r '.value')

# Set CF API endpoint
cf api https://api.bosh-lite.com --skip-ssl-validation

# Log in to CF using retrieved credentials
cf login -a https://api.bosh-lite.com --skip-ssl-validation -u admin -p "$CF_ADMIN_PASSWORD"
```
If successful, you'll see output like this:
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

### Step 5: Deploy an Example App

Create an organization and space, target them, and push an example Docker-based app:

```bash
cf create-org org && cf create-space -o org space && cf target -o org
cf push nginx --docker-image nginxinc/nginx-unprivileged:1.23.2
```
Once deployed, test the app using curl:
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
### Troubleshooting
#### ❌ Can't create VMs?

  Remove old BOSH state and credentials:

  ```bash
  rm -f ./state.json ./creds.yml
  rm -rf ~/.bosh/installations
  ```
  Then rerun:

  ```bash
  bosh create-env  [..omissis..]
  ```

#### ❌ Can't use `credhub`?

Make sure you can reach the `CredHub` VM via SSH:
`bosh -e vbox -d cf ssh credhub`

#### ❌ Can't connect to the auth server via `credhub`?
If you encounter the following error:
```bash
credhub api $CREDHUB_SERVER --ca-cert=$CREDHUB_CA_CERT
Error connecting to the auth server: "Get \"https://192.168.56.6:8443/info\": tls: failed to verify certificate: x509: certificate signed by unknown authority". Please validate your target and retry your request.
```
add `--skip-tls-validation` flag and ignore the warning

```bash
credhub api $CREDHUB_SERVER --ca-cert=$CREDHUB_CA_CERT --skip-tls-validation
Warning: The targeted TLS certificate has not been verified for this connection.
Warning: The --skip-tls-validation flag is deprecated. Please use --ca-cert instead.
Setting the target url: https://192.168.56.6:8844
```

#### ❌ Can't Push Docker Images?

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