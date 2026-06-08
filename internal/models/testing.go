package models

import "errors"

// This file contains test-only hooks that allow other internal packages
// to inject mock fetchers for the model catalog. They must NEVER be called
// from production code. They live in a separate file (not in models.go)
// to make their test-only intent explicit and to keep the production API
// surface clean.
//
// If you need to use these in a test, prefer importing them via the
// modeltest package (a thin wrapper) so the dependency direction is clear.

func SetFetchersForTest(remote map[string]remoteModelInfo, official []OfficialModel, officialErr error, remoteErr error) {
	remoteModels = newLazyFetcher(func() (map[string]remoteModelInfo, error) {
		if remoteErr != nil {
			return nil, remoteErr
		}
		if remote == nil {
			return nil, errors.New("remote unavailable")
		}
		return remote, nil
	})
	officialModels = newLazyFetcher(func() ([]OfficialModel, error) {
		if officialErr != nil {
			return nil, officialErr
		}
		if official == nil {
			return nil, errors.New("official unavailable")
		}
		return official, nil
	})
}

func ResetFetchersForTest() {
	remoteModels = newLazyFetcher(fetchRemoteModels)
	officialModels = newLazyFetcher(fetchOfficialModels)
}
