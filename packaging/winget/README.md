# WinGet packaging

This directory contains draft WinGet manifests for OCGO.

## Validate locally

```powershell
# YAML syntax check (does not require winget)
Get-ChildItem .\packaging\winget\manifests\u\UlrichZogo\OCGO\0.1.0 -Filter *.yaml | ForEach-Object { $content = Get-Content $_.FullName -Raw; $null = [System.Management.Automation.Language.Parser]::ParseInput($content, [ref]$null, [ref]$errors); if ($errors.Count -gt 0) { Write-Error "$($_.Name): $($errors -join '; ')" } else { Write-Host "$($_.Name): valid" } }
```

With winget:

```powershell
winget validate .\packaging\winget\manifests\u\UlrichZogo\OCGO\0.1.0
```

## Future submission

These manifests are intended to be submitted to:

```text
microsoft/winget-pkgs
```

This PR does not submit the manifests automatically.
