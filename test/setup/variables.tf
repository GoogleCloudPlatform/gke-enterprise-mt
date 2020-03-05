/**
 * Copyright 2019 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
variable "org_id" {
  description = "The numeric organization id"
}

variable "folder_id" {
  description = "The folder to deploy in"
}

variable "billing_account" {
  description = "The billing account id associated with the project, e.g. XXXXXX-YYYYYY-ZZZZZZ"
}

variable "prefix" {
  description = "Generic prefix for root items. Change if for some reason you need to make things unique"
  default = "Test"
}

variable "domain" {
  description = "The domain name"
  type        = string
}

variable "gsuite_admin_account" {
  description = "The account email of an admin for the domain, above"
  type        = string
}

variable "creds_path" {
  description = "Path to terraform service account credentials file"
  type        = string
}
