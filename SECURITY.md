# Security policy

## Reporting a vulnerability

Use [GitHub private vulnerability reporting](https://github.com/Dyu-36/gotack/security/advisories/new)
when it is enabled for this repository. If that page is unavailable, contact the
maintainer through [their GitHub profile](https://github.com/Dyu-36) to arrange a
private reporting channel before sending sensitive details.

Include the host revision, engine commit from `.tack-pin`, Windows version,
reproduction steps and impact. Use dummy credentials and redact tokens, API keys,
session content and local paths from logs. Avoid publishing exploit details or
credentials in public issues.

## Security boundaries

Gotack's local agent executes enabled tools with the privileges of its user.
Project Trust controls loading project-provided prompts and skills; it does not
sandbox tool execution. See [the user guide](docs/user-guide.md).

Credentials are stored by the engine and are not revealed through a stored-key
read API in the WebView. Zalo is an optional remote channel with pairing and an
allow-list; treat access to a paired chat as access to the agent's enabled tools.

Reports concerning the bundled engine should identify the pinned engine revision
so the affected component can be investigated in the correct repository.
