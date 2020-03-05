# GKE Enterprise Multi-Tenancy Platform Example.

## Prerequisites

* Install the terraform [gsuite
  provider](https://github.com/DeviaVir/terraform-provider-gsuite).
* Make sure you have an
  [organization](https://cloud.google.com/resource-manager/docs/quickstart-organizations)
  and assocaited billing account ready to go. If the account is new, be prepared
  to request higher quotas (which may require making a small payment).
* The organization must be associated with a domain. For example, if the
  organization is created from a G Suite account, you will either bring your own
  domain or purchase one. Groups in this domain will be used for identity and
  management in the GKE platform this module will set up.
* Optionally, set up organizationally-named bucket for terraform state. This
  requires verifying the domain name for your billing account. This should be
  made under the admin project (see below).
* Create a parent GCP folder. Everything will be created under this folder for
  easier management and separation from other GCP projects.
* Create a project in this parent folder. This will be used to hold the service
  account used to run this module, so cannot be bootstrapped with terraform. We
  will refer to this as the _admin project_.
* Create a "terraform" service account in the admin project, and give it the following
  roles. TODO: can these be reduced? if the service account email is passed in
  as a variable maybe and the organizationAdmin role given.
  - `resourcemanager.organizationViewer`
  - `resourcemanager.projectCreator`
  - `billing.admin`
  - `compute.xpnAdmin`
  - `compute.networkAdmin`
  - `resourcemanager.organizationAdmin`
* Set the following roles on the parent folder.
  - `browser` (used for the Shared VPC host project)
  - `resourcemanager.projectIamAdmin` (also used for the Shared VPC host project)
  - `resourcemanager.folderAdmin`
* Get a key to the service account and export it as the default application
  credentials. See the script example below for details.
* In order to correctly assign billing accounts, this `terraform` service
  account needs to be a billing administor. In my case this was automatically
  inherited, but your organization may be set up differently. The following
  steps worked for me to check.
  - Go to the [cloud console](https://pantheon.corp.google.com/) as the user who
    manages your billing account. This may be different from the user managing
    your cloud resources. For example, at Google, a testing domain may have been
    set up, linked to a Google-internal billing account. In this case you would
    want to go to the console as your `@google.com` user and not as the testing
    domain user.
  - In the console, select the admin project, navigate to Billing, and go to the
    linked billing account for that project. This will take you to the billing
    overview page.
  - Select "Account Management" in the left-hand panel. This should bring up a
    panel on the right-hand side with a "Permissions" tab. Expand the "Billing
    Account Administrator" box.
  - In my case, the `terrafrom` service account created above had automatically
    inherited this permission. In your case, you may need to click the "Add
    members" button and add it manually.
  - Good luck!
  - If you think this is overly complicated, complain loudly. Working together
    we can improve the world.
* Export terraform variables in the form of TF_VARS as shown in the example
  script below, or use your favorite var-file.
* Check that the terraform service account is configured for gsuite by following
  the instructions
  [here](https://developers.google.com/admin-sdk/directory/v1/guides/delegation). This
  will be used for the gsuite provider installed above (see also the instruction
  in that provider's readme). There are multiple steps, you have to go through
  the gsuite dashboard and authorize stuff. It's a pain, sorry. The upshot is
  that your service account (authenticated by `$CREDS_PATH`, below) will be able
  to impersonate your gsuite admin accounts (see `$GSUITE_ADMIN_EMAIL` below).
* Optionally, configure remote terraform state. These scripts can be run using
  either local or remote terraform state. To use Cloud Storage with remote
  state, make a storage bucket, preferably in your organization's domain, and do
  the following to give the `terraform` SA permissions for the terraform state
  bucket. See the `gsutil iam ch` command in the script example below.


Here is an example script that will perform the steps above. The variables
exported at the top will need to be specified.
*TEMPORARY WARNING:* I haven't tested this exact script yet. Watch out for typos and
stupid errors.
```
# The following only has to be done once, if you haven't already installed it.
GSUITE_VERSION=0.1.39
GSUITE_ARCH=linux_amd64
mkdir -p $HOME/.terraform.d/plugins
pushd $HOME/.terraform.d/plugins
curl -L https://github.com/DeviaVir/terraform-provider-gsuite/releases/download/v${GSUITE_VERSION}/terraform-provider-gsuite_${GSUITE_VERSION}_${GSUITE_ARCH}.tgz \
   | tar xzvf -
popd

# The following can be found by runing gcloud organizations list
export ORG_ID=your-organization-id
export BILLING_ID=your-billing-id
export DOMAIN=your-gsuite-domain
export GSUITE_ADMIN_EMAIL=gsuite-admin@your-gsuite-domain
# Create a folder by running 
#  gcloud resource-manager folders create --organization $ORG_ID some-folder-name
# and put the folder id here. TODO: this could probably be extracted with clever
# parsing of the folder create command.
export PARENT_FOLDER_ID=
export ADMIN_PROJECT=your-admin-project-id
export CREDS_PATH=path-to-SA-credentials-Careful-this-is-sensitive.json
# Optional
export BUCKET_NAME=some-nice-name-for-your-storage-bucket

gcloud projects create --folder $PARENT_FOLDER $ADMIN_PROJECT
gcloud config set project $ADMIN_PROJECT
gcloud iam service-accounts create terraform --project $ADMIN_PROJECT
for r in                               \
    resourcemanager.organizationViewer \
    resourcemanager.projectCreator     \
    billing.admin                      \
    compute.xpnAdmn                    \
    compute.networkAdmin               \
    resourcemanager.organizationAdmin  \
  ; do
  gcloud organizations add-iam-policy-binding $ORG_ID \
     --member=serviceAccount:terraform@${ADMIN_PROJECT}.iam.gserviceaccount.com
     --role=roles/$r
done
for r in                            \
    browser                         \
    resourcemanager.projectIamAdmin \
    resourcemanager.folderAdmin     \
  ; do
  gcloud resource-manager folders add-iam-policy-binding $PARENT_FOLDER_ID
     --member=serviceAccount:terraform@${ADMIN_PROJECT}.iam.gserviceaccount.com
     --role=roles/$r
done

for s in                                \
   admin.googleapis.com                 \
   cloudresourcemanager.googleapis.com  \
   cloudbilling.googleapis.com          \
   iamcredentials.googleapis.com        \
   iam.googleapis.com                   \
   container.googleapis.com             \
  ; do 
  gcloud services enable $s --project ADMIN_PROJECT
done

# The following is optional, if you are using remote terraform storage.
gsutil iam ch \
   serviceAccount:terraform@${ADMIN_PROJECT}.iam.gserviceaccount.com:objectAdmin \
   gs://${BUCKET_NAME}.${DOMAIN}

gcloud iam service-accounts keys create \
  --iam-account terraform@ADMIN_PROJECT.iam.gserviceaccount.com \
  $CREDS_PATH

export GOOGLE_APPLICATION_CREDENTIALS=$CREDS_PATH
export TF_VAR_organization_id=$ORG_ID
export TF_VAR_billing_account=$BILLING_ID
export TF_VAR_folder_id=$PARENT_FOLDER_ID
export TF_VAR_domain=$DOMAIN
export TF_VAR_gsuite_admin_account=$GSUITE_ADMIN_EMAIL
export TF_VAR_creds_path=$CREDS_PATH
export TF_VAR_prefix=test
```

## Debugging Notes

* The terraform@ADMIN_PROJECT service account will not have a lot of permissions
  on the ADMIN_PROJECT itself, but will be able to work with the projects that
  are created inside the testing folder, and those are the projects you're going
  to want to examine anyway. The service account will be able to `gcloud
  projects list` and that will lead to the things you actually want to examine.
