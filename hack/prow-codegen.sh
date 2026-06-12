#!/usr/bin/env bash
# Copyright 2026 The Kubernetes Authors.
# Licensed under the Apache License, Version 2.0 (the "License");

set -euo pipefail

SCRIPT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_ROOT}/.." && pwd)

cd "${REPO_ROOT}"

# Run codegen
bash ./hack/update-codegen.sh

# Run tidy
go mod tidy

# Check for changes
if [[ -z $(git status -s) ]]; then
  echo "No changes, codegen is up-to-date."
  exit 0
fi

echo "Detected changes after codegen/tidy."

# In Prow, PULL_HEAD_REF is the source branch.
# If it is copybara-sync, we try to push back.
if [[ "${PULL_HEAD_REF:-}" == "copybara-sync" ]]; then
  echo "PR is from copybara-sync. Attempting to push changes back..."
  
  git config user.name "GKE MT Prow Robot"
  git config user.email "gke-mt-prow-robot@google.com"
  
  git add .
  git commit -m "Automated codegen and go.mod tidy (Prow)"
  
  # Push back to the PR branch
  # Prow checked out the code, origin should point to the main repo.
  git push origin HEAD:${PULL_HEAD_REF}
  
  echo "Pushed changes back to ${PULL_HEAD_REF}. Failing current run to trigger rebuild."
  exit 1
else
  echo "Changes detected in non-copybara PR. Please run hack/update-codegen.sh locally and commit changes."
  git diff
  exit 1
fi
