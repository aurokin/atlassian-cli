# Confluence MVP

## API grounding

- Confluence v2 intro: https://developer.atlassian.com/cloud/confluence/rest/v2/intro/
- Confluence v1 intro: https://developer.atlassian.com/cloud/confluence/rest/v1/intro/
- Basic auth: https://developer.atlassian.com/cloud/confluence/basic-auth-for-rest-apis/
- Scoped tokens: https://support.atlassian.com/confluence/kb/scoped-api-tokens-in-confluence-cloud/

## MVP command tree

```text
confluence auth login|logout|status
confluence api
confluence resolve
confluence browse
confluence config
confluence space list|view
confluence page list|view|create|edit|children
confluence search cql
confluence status
```

## Design notes

- Prefer page IDs after URL/title resolution.
- Do not silently convert body formats.
- Require explicit `--body-format` for writes.
- Keep v1 fallback for official endpoints not covered by v2.
- Treat page editing as versioned and conflict-prone.
