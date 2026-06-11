# Scoop packaging

This directory contains the Scoop manifest for OCGO.

## Local test

```powershell
scoop install .\packaging\scoop\ocgo.json
ocgo --help
ocgo models
scoop uninstall ocgo
```

## Future bucket

Recommended bucket:

```powershell
scoop bucket add ocgo https://github.com/ulrich-zogo/scoop-ocgo
scoop install ocgo
```

The manifest is generated for the current release and can be copied into the future bucket repository.
