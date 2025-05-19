# How to deploy Cloud Foundry in AWS using cf-deployment

Deploying Cloud Foundry in AWS requires a series of steps that start by creating an AWS specific account and using different cli commands to setup the BOSH enviroment and then deploy the Cloud Foundry VMs. The deployment of Cloud Foundry in AWS carries a significant economical burden compared to the bosh-lite deployment, just by the fact that it creates 17 EC2 instances, 3 Load Balancers and a handful of Public IPs, notwithstanding the snapshots and volumes needed. Use this approach only when the bosh-lite deployment does not satisfy your needs.

# Create the bbl-user AWS account
Having said that, the first thing that you will need to do is create an AWS account using the steps described [here](https://cloudfoundry.github.io/bosh-bootloader/getting-started-aws/).

```bash
pbpaste="{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "logs:*",
                "elasticloadbalancing:*",
                "cloudformation:*",
                "iam:*",
                "kms:*",
                "route53:*",
                "ec2:*"
            ],
            "Resource": "*"
        }
    ]
}"
aws iam create-user --user-name "bbl-user"
aws iam put-user-policy --user-name "bbl-user" \
    --policy-name "bbl-policy" \
    --policy-document "$(pbpaste)"
aws iam create-access-key --user-name "bbl-user"
```

Store the access key and secret in a safe location, you will need to reference these in the next set of commands and in the future when you need to interact with the AWS resources.

## Setup the bosh environment
Before dweling into the bosh environment, make sure you have downloaded the latest `bbl`, `bosh` and `credhub` cli binaries. The `bbl` binary can be downloaded [here](https://github.com/cloudfoundry/bosh-bootloader/releases), while the `bosh` binary [here](https://github.com/cloudfoundry/bosh-cli/releases). Finally, the `credhub` binary can be downloaded [here](https://github.com/cloudfoundry/credhub-cli/releases).

Once you're downloaded both binaries and they are in your path and ready to be used, run the following commands to export the environment variables to be reused in the installation. Replace the variables in `<>` with your specific values.

```bash
export access_key_id=<AWS_BBL_USER_ACCESS_KEY_ID>
export access_key_secret=<AWS_BBL_USER_ACCESS_KEY_SECRET>
export region=<AWS_REGION>
export ENV_NAME=aws
export SYSTEM_DOMAIN=bosh-$ENV_NAME.com
```

## Generate a self signed certificates for for Load Balancer:
The AWS deployment deploys a Load Balancer as part of the ingress communication between the client and the Cloud Foundry infrastructure. Here we will create a self signed certificate that matches the domain we plan to use:

```bash
openssl req -x509 -newkey rsa:2048 -nodes -keyout key.pem -out cert.pem \
   -subj "/C=US/ST=California/L=San Francisco/O=Your Company/OU=Your Department/CN=*.$SYSTEM_DOMAIN" \
   -addext "subjectAltName=DNS:$SYSTEM_DOMAIN,DNS:*.$SYSTEM_DOMAIN"
```

Validate that its contents are correct:

```
openssl x509 -noout -text -in 'cert.pem'
```

This should render something similar to this:

```bash
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            22:15:87:45:84:01:f3:67:be:b5:9d:c1:78:57:da:a6:ee:f0:91:52
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: C=US, ST=California, L=San Francisco, O=Your Company, OU=Your Department, CN=*.bosh-aws.com
        Validity
            Not Before: May 19 15:14:00 2025 GMT
            Not After : Jun 18 15:14:00 2025 GMT
        Subject: C=US, ST=California, L=San Francisco, O=Your Company, OU=Your Department, CN=*.bosh-aws.com
        Subject Public Key Info:
            Public Key Algorithm: rsaEncryption
                Public-Key: (2048 bit)
                Modulus:
                    00:c2:61:a6:13:ae:20:b9:25:89:84:64:d1:69:0f:
                    1b:d2:68:46:92:54:64:03:64:94:1b:28:aa:1b:3b:
                    2b:3f:11:9d:20:3b:f4:b1:a7:88:31:e4:ae:cc:9d:
                    b4:d7:85:55:96:7e:ec:bd:6a:63:ab:64:03:7c:db:
                    ee:71:10:36:aa:0d:a1:c6:38:b6:90:40:c9:8b:75:
                    9f:e7:7c:52:2e:0d:26:6e:f9:55:8c:71:f2:1c:f9:
                    6d:5f:c2:6b:17:9b:89:fa:ae:0f:68:e2:af:b7:b3:
                    15:ad:70:6c:a8:cd:a3:c3:a2:42:ea:f3:5d:f5:04:
                    2e:ea:ac:99:f8:8f:26:4f:0c:fc:dc:2e:00:62:e3:
                    d7:6a:ba:ae:f6:c5:f0:c0:5d:d8:ce:8d:41:f9:5d:
                    2c:6b:59:ab:f1:eb:62:f4:11:15:eb:17:f7:13:97:
                    18:c6:f7:b5:74:75:0f:e3:c1:22:2a:76:65:e9:9a:
                    4e:2e:82:4e:92:1a:03:8d:47:91:84:8e:d0:97:a2:
                    6b:c0:92:67:12:41:e2:d9:17:de:9f:04:2d:a9:a4:
                    da:dd:e0:2b:18:cd:88:0f:fe:e8:5a:e5:59:79:d6:
                    45:59:ea:74:5f:3f:5c:a2:33:cf:15:3a:a6:ba:e2:
                    0c:7b:f9:15:7d:4c:fb:1f:75:a9:25:2f:d4:2b:2a:
                    fa:41
                Exponent: 65537 (0x10001)
        X509v3 extensions:
            X509v3 Subject Key Identifier:
                87:AB:71:2C:88:70:24:DD:05:E9:10:41:D3:49:9D:69:41:53:84:48
            X509v3 Authority Key Identifier:
                87:AB:71:2C:88:70:24:DD:05:E9:10:41:D3:49:9D:69:41:53:84:48
            X509v3 Basic Constraints: critical
                CA:TRUE
            X509v3 Subject Alternative Name:
                DNS:bosh-aws.com, DNS:*.bosh-aws.com
    Signature Algorithm: sha256WithRSAEncryption
    Signature Value:
        57:bf:99:a1:c1:a9:93:25:cc:39:1f:4c:0a:be:93:7f:c4:bc:
        82:24:30:64:1c:1c:3c:93:3b:28:7a:9f:1f:2c:ca:4b:83:71:
        4c:ef:b6:03:ed:a6:54:e6:49:c6:95:97:21:1a:4b:6d:d8:9f:
        ee:7e:fa:a7:c8:7e:cb:22:ef:0d:b3:dd:e2:52:0f:42:a7:17:
        cb:c5:bb:63:a0:18:21:60:9b:7e:f5:82:1e:9c:bf:d4:3a:95:
        3a:67:c9:54:7f:4e:b3:1f:03:d5:3b:c8:a9:97:40:e9:25:e1:
        84:09:48:ab:72:bd:19:1d:e8:42:bc:a2:08:67:bb:88:ab:ea:
        1e:be:0a:a6:85:3a:c9:64:a2:f7:2b:41:86:0a:a2:f3:32:6f:
        f0:b6:2e:6f:48:b6:9b:f3:11:d6:e0:93:91:1a:8c:c4:5a:3e:
        79:33:87:c8:b5:9c:21:13:a4:4e:e1:28:3f:17:ad:4d:d0:29:
        12:ca:03:c4:3a:61:8b:0d:fa:57:26:1d:6e:0b:03:31:87:5d:
        84:eb:b8:21:b8:a1:55:f0:f5:ab:5e:fd:89:3d:2a:2d:18:5e:
        9e:f6:1f:4b:11:68:77:ec:26:77:c4:0c:de:85:a2:52:91:c7:
        f7:6e:21:67:bb:93:56:24:17:75:e6:64:09:f9:d9:ae:27:c8:
        55:6a:f0:a3

```

## Create the Bosh environment (jumpbox + director)
The following command will configure the environment where the bosh director and the jumpbox VMs are instantiated and configured with the `bbl` command:

```bash
bbl plan \
    --aws-access-key-id $access_key_id \
    --aws-secret-access-key $access_key_secret \
    --aws-region $region \
    --iaas aws \
    --lb-type cf \
    --lb-cert cert.pem \
    --lb-key key.pem \
    --lb-domain $SYSTEM_DOMAIN \
    --name $ENV_NAME
```

The following command will instruct `bbl` to execute the configured enviroment defined in the previous command. It is possible to combine both commands so that the `plan` command is no longer necessary and just running `bbl up ...` with the same parameters in the `plan` command is enough.

```bash
bbl up \
    --aws-access-key-id $access_key_id \
    --aws-secret-access-key $access_key_secret \
    --aws-region $region \
    --iaas aws \
    -n
```

Note that this command takes about 30 minutes to complete. Once completed, run the following command to ensure that the bosh director has been successfully deployed:

```bash
eval "$(bbl print-env)"; bosh log-in
```

You should see something like this:

```bash
$> eval "$(bbl print-env)"; bosh log-in
Successfully authenticated with UAA

Succeeded
```

## Deploy Cloud Foundry in AWS
We're half way through, at this point there should be 2 EC2 instances in AWS that are the jumpbox and the bosh director. The `vars/` subdirectory contains the terraform and other sensitive files to connect to the bosh director as well as to authenticate through the jumpbox.

### Clone the cf-deployment repository:
Before installing Cloud Foundry, you will need to clone the cf-deployment repository, which contains the configuration to setup Cloud Foundry in a development environment:

```
git clone https://github.com/cloudfoundry/cf-deployment.git
```

### Validate OS Stemcell based on cf-deployment

The OS stemcell defined in bbl might not match the one in the cf-deployment.yaml. During the bosh director deployment, bbl will upload its preconfigured version, but it might not match the one defined in the `cf-deployment/cf-deployment.yml` file. Check the output from the `bbl up` command and locate the line where it mentions which stemcell version is uploading:

```bash
...
Finished installing CPI (00:03:30)

Uploading stemcell 'bosh-aws-xen-hvm-ubuntu-jammy-go_agent/1.822'... Finished (00:04:23)

Started deploying
...
```

Compare it with the one in the `cf-deployment/cf-deployment.yml` file:

```
tail -n4 cf-deployment/cf-deployment.yml
```

Example:
```
$> tail -n4 cf-deployment/cf-deployment.yml
stemcells:
- alias: default
  os: ubuntu-jammy
  version: "1.824"
```

If they don't match, upload the stemcell specified in the cf-deployment.yml. The following command is also retrieveable from the bosh.io website https://bosh.io/stemcells/bosh-aws-xen-hvm-ubuntu-jammy-go_agent )
```
bosh upload-stemcell --sha1 c040c8101a39f3118b4b2155262d10bb4b9c2d57 "https://bosh.io/d/stemcells/bosh-aws-xen-hvm-ubuntu-jammy-go_agent?v=1.824"

```

You can also override the one in the `cf-deployment.yml` or specify the version in `bbl plan`, whichever solution works better for you.

## Deploy CF
At this point, we're ready to deploy Cloud Foundry. Run the following command to start the deployment:

```bash
bosh -d cf deploy cf-deployment/cf-deployment.yml \
-o cf-deployment/operations/aws.yml \
-o cf-deployment/operations/use-latest-stemcell.yml \
-o cf-deployment/operations/scale-to-one-az.yml \
-v system_domain=$SYSTEM_DOMAIN \
-n
```

This command will take roughly 90 minutes to complete.

### Configure your local environment to be able to communicate with the Cloud Foundry instance
You will need to configure your `/etc/hosts` to be able to communicate with the Cloud Foundry services using the system domain we provided. First we'll retrieve the router LB with the `aws` cli and then we'll resolve its domain name to one of its IPs so we can add an entry in the /etc/hosts with the Cloud Foundry subdomains:

```
export LB_DNS_NAME=$(aws elb describe-load-balancers --load-balancer-name $ENV_NAME-cf-router-lb --query 'LoadBalancerDescriptions[0].DNSName' --output text)
export LB_IP=$(host $LB_DNS_NAME |tail -n 1 |awk '{print $4}')
echo $LB_IP $SYSTEM_DOMAIN api.$SYSTEM_DOMAIN login.$SYSTEM_DOMAIN uaa.$SYSTEM_DOMAIN doppler.$SYSTEM_DOMAIN log-stream.$SYSTEM_DOMAIN log-cache.$SYSTEM_DOMAIN | sudo tee -a /etc/hosts
```

Example:
```
$> echo $LB_IP $SYSTEM_DOMAIN api.$SYSTEM_DOMAIN login.$SYSTEM_DOMAIN uaa.$SYSTEM_DOMAIN doppler.$SYSTEM_DOMAIN log-stream.$SYSTEM_DOMAIN log-cache.$SYSTEM_DOMAIN | sudo tee -a /etc/hosts
$> cat /etc/hosts |tail -n 1
18.219.57.18 bosh-aws.com api.bosh-aws.com login.bosh-aws.com uaa.bosh-aws.com doppler.bosh-aws.com log-stream.bosh-aws.com log-cache.bosh-aws.com
```

Validate that the API server connectivity is up and working by pointing the Cloud Foundry CLI to the API server via the Load Balancer.

```
cf api https://api.$SYSTEM_DOMAIN --skip-ssl-validation
```

Example:
```
$> cf api https://api.bosh-aws.com --skip-ssl-validation
Setting API endpoint to https://api.bosh-aws.com...
OK

API endpoint:   https://api.bosh-aws.com
API version:    3.193.0

Not logged in. Use 'cf login' or 'cf login --sso' to log in.
```

Retrieve the CF admin password, which is located in credhub.

```
export CF_ADMIN_PASSWORD=$(credhub get -n /bosh-$ENV_NAME/cf/cf_admin_password -q)
```

Example:
```
$> export CF_ADMIN_PASSWORD=$(credhub get -n /bosh-$ENV_NAME/cf/cf_admin_password -q)
$> echo $CF_ADMIN_PASSWORD
EtT6hi17Heo6sZdMU3EOguRmt1QdTG
```

And finally, login to Cloud Foundry:

```
cf login -a https://api.$SYSTEM_DOMAIN -u admin -p "$CF_ADMIN_PASSWORD" --skip-ssl-validation
```

Example:

```
$> cf login -a https://api.bosh-aws.com -u admin -p "$CF_ADMIN_PASSWORD" --skip-ssl-validation
API endpoint: https://api.bosh-aws.com


Authenticating...
OK

Targeted org system.

API endpoint:   https://api.bosh-aws.com
API version:    3.193.0
user:           admin
org:            system
space:          No space targeted, use 'cf target -s SPACE'
```

Congratulations! you have installed Cloud Foundry in AWS!


# Deleting all resources
The most effective way to delete all resources is to run the following command in the directory where you run the `bbl up` command. First we will remove the Cloud Foundry instances using bosh

```bash
bosh -d cf delete-deployment --force -n
```

Which should render something like this:

```bash
$> bosh -d cf delete-deployment --force -n
Using environment 'https://10.0.0.6:25555' as client 'admin'

Using deployment 'cf'

Task 34

Task 34 | 20:13:07 | Deleting instances: smoke-tests/6d246990-de7b-41f5-a82d-368fb5db1d6b (0)
Task 34 | 20:13:07 | Deleting instances: nats/1745b959-6f87-4938-8282-7d299c2d4228 (0)
Task 34 | 20:13:07 | Deleting instances: router/9a480091-7cdc-4c37-95e8-58966ea44f0e (0)
Task 34 | 20:13:07 | Deleting instances: doppler/1aed652b-85de-427e-a7c6-59529e813e5e (0)
...
...
Task 34 Started  Mon May 19 20:13:07 UTC 2025
Task 34 Finished Mon May 19 20:15:17 UTC 2025
Task 34 Duration 00:02:10
Task 34 done

Succeeded
```

Note that there is no correlation between the task number and the task itself. It's just a counter that is used by BOSH to keep track of its actions.

And finally remove the jumpbox, bosh and all remaining AWS resources:

```bash
bbl destroy \
    --aws-access-key-id $access_key_id \
    --aws-secret-access-key $access_key_secret \
    --aws-region $region \
    --iaas aws \
    -n
```

This will render an output like the following:

```bash
$> bbl destroy \
    --aws-access-key-id $access_key_id \
    --aws-secret-access-key $access_key_secret \
    --aws-region $region \
    --iaas aws \
    -n
step: cleaning up director resources
step: destroying bosh director
...
...
Cleaning up rendered CPI jobs... Finished (00:00:00)

Succeeded
step: generating terraform template
step: generating terraform variables
step: terraform init
step: terraform destroy
step: finished destroying infrastructure
```