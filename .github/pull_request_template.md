## Summary

<!-- Explain what this PR changes and why. -->

## Scope

- [ ] Runtime behavior
- [ ] CLI behavior
- [ ] Daemon/proxy
- [ ] Codex/Claude/Desktop config
- [ ] Release/install/distribution
- [ ] Docs only
- [ ] Repository governance

## Validation

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `go build ./cmd/ocgo`
- [ ] `go test ./internal/e2e -run E2E -v`
- [ ] Other:

## Safety / compatibility

- [ ] No real user HOME/config is modified by tests
- [ ] No secrets are printed
- [ ] No references to the upstream owner are introduced
- [ ] Existing release/install workflows are preserved

## Notes

<!-- Anything reviewers should know. -->
