---
name: Bug report
about: Report a reproducible defect
title: "[bug] "
labels: bug
---

## What happened

A clear description of the defect.

## Expected

What you expected to happen.

## Reproduce

Exact commands and outputs.

```sh
# commands here
```

## Environment

- OS and version:
- Go version (`go version`):
- Beacon commit (`git rev-parse HEAD`):
- CLI built from this repo? yes / prebuilt release
- Database filesystem: local (ext4/apfs/ntfs) / synchronized-or-network mount

## Did you check the docs?

- [ ] [Troubleshooting](../docs/optimization/troubleshooting.md)
- [ ] [Limitations](../docs/optimization/limitations.md) — is this a known, documented gap?

## Is this a security issue?

If yes, **do not** file it here. Follow [SECURITY.md](../SECURITY.md).
