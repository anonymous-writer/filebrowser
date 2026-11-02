# URL Download extension (development patch)

This patch adds a **URL Download** action to the FileBrowser Quantum create-actions menu.
It requests a public HTTP(S) URL from the server and writes the response into the currently
open folder through the existing `ResourcePostHandler` path, preserving its permission,
quota, and no-overwrite checks.

## Changed files

- `frontend/src/components/ContextMenu.vue` — adds the menu action and passes the current path/source.
- `frontend/src/components/prompts/Prompts.vue` — registers the prompt component.
- `frontend/src/components/prompts/URLDownload.vue` — URL input, submit, and cancel UI.
- `backend/internal/web/httpRouter.go` — registers `POST /resources/url-download`.
- `backend/internal/web/url_download.go` — server-side fetch and stream to existing resource handler.

## Important limitations

This is an **experimental development patch**, not a production-ready release. The UI currently
shows an activity indicator, not byte percentage/speed. It supports direct HTTP(S) file URLs;
websites requiring login, JavaScript, cookies, or special download flows are not supported.
The server rejects non-public resolved IP addresses and disables environment proxies to keep
DNS/IP checks in the server's own connection path. Additional security review and tests are
required before merging or exposing this endpoint to untrusted users.

## Build and test

Use the Node.js version required by `frontend/.nvmrc` and the Go version declared in
`backend/go.mod`. Then run the project's normal frontend build/typecheck and backend tests.

The implementation was not successfully compiled in the preparation environment: the installed
Go toolchain is older than the project's required Go version, and the frontend dependency install
exceeded the available execution window. Please test on a matching development environment.
