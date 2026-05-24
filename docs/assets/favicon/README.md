# Favicon / icon set

Browser and touch icons derived from the project logo
([`../atlassian-cli-logo.png`](../atlassian-cli-logo.png)), cropped to the robot
mascot (the wordmark is illegible at favicon sizes) and centered on a
transparent square.

| File | Size | Use |
|---|---|---|
| `favicon.ico` | 16/32/48 | Legacy `/favicon.ico` fallback |
| `favicon-16x16.png` | 16 | Browser tab (transparent) |
| `favicon-32x32.png` | 32 | Browser tab / bookmarks (transparent) |
| `apple-touch-icon.png` | 180 | iOS home-screen (solid white background — iOS composites transparency over black) |
| `icon-192.png` | 192 | Android / PWA manifest (transparent) |
| `icon-512.png` | 512 | PWA manifest / high-DPI master (transparent) |

> There is no published web frontend for this project yet, so nothing serves
> these today — they are committed ready for a future docs site or GitHub Pages.

When a site exists, reference them from the page `<head>`:

```html
<link rel="icon" href="/favicon.ico" sizes="any">
<link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png">
<link rel="icon" type="image/png" sizes="16x16" href="/favicon-16x16.png">
<link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">
```

## Regenerating

These are generated from the logo with Pillow. To rebuild after the logo
changes, re-run the crop+resize steps (mascot crop box `(344,160,677,551)`,
letterboxed onto a transparent square, then resized with LANCZOS to each size;
`favicon.ico` holds the 16/32/48 set). The detection that found the crop box is
an ink-bounding-box scan over the upper image band — see the PR that introduced
these assets.
