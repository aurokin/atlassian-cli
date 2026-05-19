# Confluence MVP

## API grounding

- Confluence v2 intro: https://developer.atlassian.com/cloud/confluence/rest/v2/intro/
- Confluence v1 intro: https://developer.atlassian.com/cloud/confluence/rest/v1/intro/
- Basic auth: https://developer.atlassian.com/cloud/confluence/basic-auth-for-rest-apis/
- Scoped tokens: https://support.atlassian.com/confluence/kb/scoped-api-tokens-in-confluence-cloud/

## MVP command tree

```text
atl-conf auth login|logout|status
atl-conf api
atl-conf resolve
atl-conf browse
atl-conf config
atl-conf space list|view
atl-conf page list|view|create|edit|children
atl-conf page comment list|view|create|edit|delete
atl-conf page label list|add|remove
atl-conf attachment list|download
atl-conf search cql
atl-conf status
```

## Design notes

- Prefer page IDs after URL/title resolution.
- Do not silently convert body formats.
- Require explicit `--body-format` for writes.
- Keep v1 fallback for official endpoints not covered by v2.
- Treat page and footer-comment editing as versioned and conflict-prone.
- Footer comments only; inline comments are out of scope.
- Page labels are written through the v1 content-label surface.
- Attachment download requires an explicit `--out` destination.
