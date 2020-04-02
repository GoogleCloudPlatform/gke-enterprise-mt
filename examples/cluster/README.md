# GKE Enterprise Multi-Tenancy Cluster Example.

This example creates two private GKE clusters in a shared VPC following the
enterprise multi-tenant best practices as appropriate.

The VPC host project is created in the given network adminstrator folder, and
the cluster service projects are created in the given cluster adminstrator
folder.

G Suite features, such as authenticator security groups, are not enabled.

## Notes

* Double-check no race with service activation. Wasn't that supposed to be
  fixed? Or did I specify the wrong module version?  

## Prerequisites

* Make sure you have an
  [organization](https://cloud.google.com/resource-manager/docs/quickstart-organizations)
  and assocaited billing account ready to go. If the account is new, be prepared
  to request higher quotas (which may require making a small payment).
* Create a parent GCP folder. Everything will be created under this folder for
  easier management and separation from other GCP projects.
* Export the following terraform variables (or use your favorite tfvars file),
  filling in the organization id, billing account, and a folder to hold all
  created projects.
```
export TF_VAR_organization_id=<your organization id, eg 123456>
export TF_VAR_billing_account=<your-billing-id, eg XXXXXX-YYYYYY-ZZZZZZ>
export TF_VAR_folder_id=<your-folder-id, eg 123456>
```
