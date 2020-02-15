# GKE Enterprise Multi-Tenancy Module

## Prerequisites

* Make sure you have an
  [organization](https://cloud.google.com/resource-manager/docs/quickstart-organizations)
  and assocaited billing account ready to go. If the account is new, be prepared
  to request higher quotas (which may require making a small payment).
* The organization must be associated with a domain. For example, if the
  organization is created from a G Suite account, you will either bring your own
  domain or purchase one. Groups in this domain will be used for identity and
  management in the GKE platform this module will set up.
* Set up organizationally-named bucket for terraform state. This requires
  verifying the domain name for your billing account. This should be made under
  the admin project (see below).
* In the domain, create the following goups. These are used to bind RBAC roles
  to groups and isolate user management from the platform admin team.
```
hmt-auditor
hmt-cluster-admin
hmt-folder-admin
hmt-network-admin
hmt-ns1-admin
hmt-ns1-dev
hmt-ns2-admin
hmt-ns2-dev
hmt-org-admin
hmt-security-admin
hmt-tenant-admin
hmt-tenant-dev
```
* Create parent GCP folder, and create an admin project in that
  folder. Everything made by this module will live under that folder. `gcloud
  resource-manager folders create --organization ORG-ID`, `gcloud projects
  create --folder ADMIN_PROJECT`). Make all this the default in `gcloud config`.
* Create a "terraform" service account for admin project (`gcloud iam service-accounts create
  terraform --project ADMIN_PROJECT`)
* Give service account the following roles on the organization (`gcloud
  organizations add-iam-policy-binding ETC`).
  - `roles/resourcemanager.organizationViewer`
  - `roles/resourcemanager.projectCreator`
  - `roles/billing.admin`
  - `roles/compute.xpnAdmin`
  - `roles/compute.networkAdmin`
  - `roles/resourcemanager.organizationAdmin`
* The following roles are set on the folder, using `gcloud resource-manager folders add-iam-policy-binding`.
  - `roles/browser` (used for the Shared VPC host project)
  - `roles/resourcemanager.projectIamAdmin` (also used for the Shared VPC host project)
  - `roles/resourcemanager.folderAdmin`
* From the managing account for the billing account used with your organization,
    confirm this admin SA as a billing admin. Depending on how you organize
    things, the owner of the billing account need not be in your
    organization. It may be that the billing admin role is automatically
    inherited from your organization admin account.
* These scripts can be run using local or remote terraform state. For using
  Cloud Storage with remote state, make a storage bucket, preferably in your
  organization's domain, and do the following to give the terraform SA permissions for the terraform state bucket:
```
$ gsutil iam ch serviceAccount:terraform@ADMIN_PROJECT.iam.gserviceaccount.com:objectAdmin gs://BUCKET-NAME.mattcary.info
```
  It will also be helpful to add your user to the service account with `roles/iam.serviceAccountTokenCreator`
  so that you will be able to impersonate it for unwedging things (like deleting projects).
* Enable services in the admin project:
```
for s in \
   cloudresourcemanager.googleapis.com  \
   cloudbilling.googleapis.com          \
   iamcredentials.googleapis.com        \
   iam.googleapis.com                   \
   container.googleapis.com             
do gcloud services enable $s --project ADMIN_PROJECT; done
```
* Download credentials for the service account and use those for the
  `SERVICE_ACCOUNT_JSON` environment variable as described in
  [CONTRIBUTING](../../CONTRIBUTING.md); export the key
   as well as `GOOGLE_APPLICATION_CREDENTIALS`
```
$ gcloud iam service-accounts keys create --iam-account terraform@ADMIN_PROJECT.iam.gserviceaccount.com /path/to/credentials.json
$ export GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json
```

## Debugging Notes

* The terraform@ADMIN_PROJECT service account will not have a lot of permissions
  on the ADMIN_PROJECT itself, but will be able to work with the projects that
  are created inside the testing folder, and those are the projects you're going
  to want to examine anyway. The service account will be able to `gcloud
  projects list` and that will lead to the things you actually want to examine.
