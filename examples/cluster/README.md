# GKE Enterprise Multi-Tenancy Cluster Example.

This example creates two private GKE clusters in a shared VPC following the
enterprise multi-tenant best practices as appropriate. The clusters are
imaginatively named `x` and `y`.

The VPC host project is created in the given network adminstrator folder, and
the cluster service projects are created in the given cluster adminstrator
folder.

A string prefix to use with project names is also required. See
[variables.tf](variables.tf) for details, as well as optional parameters such as
the region. Some of these parameters are specific to the clusters and have `x`
and `y` suffixes.

G Suite features, such as authenticator security groups, are not enabled.

## Prerequisites

* Make sure you have an
  [organization](https://cloud.google.com/resource-manager/docs/quickstart-organizations)
  and associated billing account ready to go. If the account is new, be prepared
  to request higher quotas (which may require making a small payment).
* Create a parent [GCP folder](https://cloud.google.com/resource-manager/docs/creating-managing-folders). Everything will be created under this folder for
  easier management and separation from other GCP projects.
* If you need a service account to run terraform under, you may create a project
  under this folder with the following APIs enabled.
  - `cloudresourcemanager.googleapis.com`
  - `cloudbilling.googleapis.com`
  - `serviceusage.googleapis.com`
  - `iam.googleapis.com`
  - `container.googleapis.com`
  Then create a service account in that project with the following roles.
  - `roles/resourcemanager.projectCreator`
  - `roles/billing.projectManager`
  - `roles/resourcemanager.folderAdmin`
  Go into your billing account management and add the service account as a
  Billing Account Adminstrator (under Permissions in Account Management).
* Export credentials for terraform to run under into `GOOGLE_CREDENTIALS`. For
  example, if you used a service account `account@somedomain.com` with
  appropriate permissions to manage the parent folder, execute the following.
```
gcloud iam service-accounts keys create --iam-account account@somedomain.com creds.json
export GOOGLE_CREDENTIALS=$(< cred.json)
```

## Usage
* Export the following terraform variables (or use your favorite tfvars file),
  filling in the organization id, billing account, and a folder to hold all
  created projects. Add any other [variables](variables.tf) where the default
  isn't appropriate.
```
export TF_VAR_organization_id=<your organization id, eg 123456>
export TF_VAR_billing_account=<your-billing-id, eg XXXXXX-YYYYYY-ZZZZZZ>
export TF_VAR_cluster_folder_id=<your-cluster-folder-id, eg 123456>
export TF_VAR_network_folder_id=<your-network-folder-id, eg 123456>
export TF_VAR_prefix=<prefix for your projects>
```
* From this diredtory, run the standard terraform apply loop:
```
terraform init
terraform plan
terraform apply
```

