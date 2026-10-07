# Redis URI diagnostic redaction

Status: implemented, draft review; no release or runtime adoption.
Owner: completed SPEC-001 security-hardening baseline; this bounded CR does not reopen baseline readiness. Source: main `57b9e964d57b3e9e1df09163be21470541441cc6`. Open draft #22/dev retains the same parser error path and is not modified.

REQ-URI-001: malformed credential-bearing Redis URIs must fail with nil connection options without exposing userinfo through error text, verbose formatting or wrapped causes.
REQ-URI-002: successfully parsed URI options and existing invalid-input rejection remain unchanged.

Trust boundary: operator-supplied connection URI to caller diagnostics/logs. This is a bounded credential-disclosure defect when malformed configuration errors are logged; no live credential or deployment was examined.
