---
name: manage-pull-requests
version: 1.0.0
description: A skill for managing pull requests in Forgejo repositories using the forgejo-cli.
license: Apache-2.0
compatibility: |
  Requires forgejo-cli (https://codeberg.org/forgejo-contrib/forgejo-cli/wiki/PRs).
  Agent must have network access to the Forgejo API.
metadata:
  category: development-tools
  tags: [git, forgejo, pull-requests, automation]
---

# Manage Pull Requests

This skill provides capabilities for managing pull requests in Forgejo repositories.

## Supported Actions

- **List PRs**: List open, closed, or all pull requests
- **View PR**: View details of a specific pull request
- **Create PR**: Create a new pull request
- **Merge PR**: Merge an open pull request
- **Close PR**: Close a pull request without merging
- **Comment**: Add a comment to a pull request
