# Brand assets

Logo and derived imagery for `atlassian-cli`.

| Asset | Size | Use |
|---|---|---|
| `atlassian-cli-logo.png` | 1024×1024, transparent | Primary logo. Shown at the top of the [root README](../../README.md); source for the derived assets below. |
| `social-preview.png` | 1280×640 | GitHub **social preview** (Settings → General → Social preview) and a future docs site's `og:image` / link-unfurl card. Logo on white at GitHub's 2:1 ratio. |
| [`favicon/`](favicon/) | 16–512 | Browser tab + touch/PWA icons, cropped to the robot mascot. See [favicon/README.md](favicon/README.md). |

All are derived from `atlassian-cli-logo.png`. To rebuild after the logo
changes, regenerate `social-preview.png` (trim the logo's transparent margins,
then fit it onto a 1280×640 white canvas with padding) and the `favicon/` set
(see that folder's README).
