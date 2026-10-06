# Pi model catalog

This package fetches the public Pi model catalog from [`https://pi.dev/api/models`](https://pi.dev/api/models). The API is an object keyed by provider ID and then model ID. The package keeps the current raw snapshot embedded so the application can start with model metadata when the endpoint and disk cache are unavailable.

## Embedded snapshot

- Retrieved: 2026-10-06
- Endpoint `Last-Modified`: 2026-10-06 12:41:33 UTC
- Pi catalog revision: `sha256-77c068ceaa2d5cb540081aeb7f0af23e5eb161b105e6b1c2c7164739e6ab6492`
- HTTP ETag: `W/"5ee1534d3b341b325d6a3f1f7437077e"`
- Raw snapshot SHA-256: `6007227af64ac3ebe7226743384ea1e671a91ee3f870dced400fd3240d1b6688`
- Size: 872,567 bytes; 42 provider keys, including providers with no chat models

The catalog is provider and model metadata published by Pi. Its contents change independently of this embedded copy. Runtime refreshes use a five-minute default TTL and conditional HTTP requests; an expired cache remains available offline. The endpoint's revision and minimum-client-version headers are stored with the disk cache for diagnostics and revalidation.

Pi is MIT-licensed. The upstream license text and attribution are included in [`LICENSE.pi`](LICENSE.pi). Source: [earendil-works/pi LICENSE](https://raw.githubusercontent.com/earendil-works/pi/main/LICENSE).
