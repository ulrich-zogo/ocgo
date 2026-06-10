# Release rollback

## When to rollback

Rollback may be needed if:

- a release artifact is corrupted;
- checksums are wrong;
- Homebrew formula points to a broken asset;
- Homebrew install fails;
- a critical runtime issue is found immediately after release.

## Rollback strategy

Prefer publishing a fixed release over deleting a release that Homebrew already references.

Recommended order:

1. Publish a new patch tag, for example `v0.1.1`.
2. Let the release workflow update GitHub Release assets and Homebrew formula.
3. Run `release-smoke` against the new tag.
4. Only delete the broken release after the tap no longer references it.

## Emergency delete

If a bad test release must be removed:

```bash
gh release delete v0.0.0-test.1 --repo ulrich-zogo/ocgo --yes
git push origin :refs/tags/v0.0.0-test.1
git tag -d v0.0.0-test.1
```

Before deleting, verify that Homebrew no longer points to the deleted tag:

```bash
gh repo clone ulrich-zogo/homebrew-tap /tmp/homebrew-tap-check
grep -R "v0.0.0-test.1" /tmp/homebrew-tap-check/Formula || true
```

## Verify current Homebrew formula

```bash
git clone https://github.com/ulrich-zogo/homebrew-tap /tmp/homebrew-tap-check
cd /tmp/homebrew-tap-check
cat Formula/ocgo.rb
```

## Validate release after rollback

Run:

```bash
gh workflow run release-smoke.yml --repo ulrich-zogo/ocgo -f tag=v0.1.1
gh run watch --repo ulrich-zogo/ocgo
```
