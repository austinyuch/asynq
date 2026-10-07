# Design

Replace the error wrapping at the net/url parse boundary with a constant diagnostic. Both url.Error.URL and its underlying parse cause can carry input bytes; clearing just the outer URL is insufficient. Do not retain the original error chain. Successful parsing and all later validation remain untouched. This intentionally removes undocumented errors.As/Unwrap access to net/url diagnostics; the public contract promises a non-nil error, not a concrete cause type.

Falsification: if a malformed URI containing the synthetic credential canary appears in Error(), %+v, or a wrapped cause after this change, REQ-URI-001 fails. Existing ParseRedisURI tables prove the successful/invalid-input contract.

No dependency upgrade, schema/proto regeneration, Redis state change, release or deployment.
