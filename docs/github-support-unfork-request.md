# GitHub Support request — detach fork network

## Purpose

This document contains the support request to ask GitHub to detach `ulrich-zogo/ocgo` from its fork network.

The goal is to keep the existing repository URL and history while removing the "forked from ..." relationship.

## Repository

```text
https://github.com/ulrich-zogo/ocgo
```

## Request

```text
Hello GitHub Support,

Please detach the repository ulrich-zogo/ocgo from its fork network
and make it a standalone repository.

This repository is now maintained as the primary project repository.
We want to preserve the current repository URL, pull requests, issues,
releases, stars, workflows, and settings, but remove the visible fork
relationship from the original upstream repository.

Repository:
https://github.com/ulrich-zogo/ocgo

I am the repository owner/admin.

Thank you.
```

## After GitHub Support confirms

Run:

```bash
scripts/audit-repo-governance.sh
```

Expected result:

```text
fork: false
parent: none
source: none
```
