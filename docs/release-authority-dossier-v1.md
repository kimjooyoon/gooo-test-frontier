# Release authority dossier v1

The repository's release manifest is not its own authority. The GitHub
repository immutable-releases setting and the GitHub Releases API `immutable`
field are authoritative over the manifest's `immutable` claim.

The historical v0.1.0 release is permanently preserved as a counterexample:

- Release ID: `379770450`
- Tag: `v0.1.0`
- Tag object: `648a114a98db27d61e70f2063586deb8e1cf0d7a`
- Target commit: `434776a02bef579cd175e8f1e29be027a9c289cc`
- Manifest claim: `immutable=true`
- GitHub API result: `immutable=false`
- Classification: `REFUTED`
- Reason: `SELF_ASSERTED_IMMUTABILITY_CONTRADICTED_BY_PLATFORM`

The repository setting was then enabled through GitHub's immutable-releases
API. A later annotated `v0.1.1` release is the only release eligible for a
successful immutable-release proof. Its Actions release audit must verify the
repository setting, the GitHub API `immutable=true`, the annotated tag object
and target, the manifest, `SHA256SUMS`, and every GitHub asset digest.

The direct-main commit `7281ead` is recorded as CI-only
(`.github/workflows/release.yml`) and is not a substantive product mutation.
A substantive direct-main mutation would be `REFUTED` by the development-
process gate.
