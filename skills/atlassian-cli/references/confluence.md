# Confluence Reference

Use `atl-conf` for Confluence Cloud or Confluence Data Center work.

## API Version Note

`atl-conf` is intentionally mixed-version:

- REST v2 backs spaces, pages, blogposts, comments, and many attachments flows.
- REST v1 backs CQL search, current-user status, label writes, and attachment upload fallbacks.

For scoped tokens or OAuth, make sure the credential has the scopes needed for
the v1 and v2 endpoints the command uses.

## Spaces And Pages

```bash
atl-conf space list --site work --json='*'
atl-conf space view DEV --site work --json='*'

atl-conf page list --space DEV --site work --limit 50 --json='*'
atl-conf page view 123456 --site work --json='*'
atl-conf page children 123456 --site work --json='*'
atl-conf page ancestors 123456 --site work --all --json='*'
atl-conf page versions 123456 --site work --all --json='*'
```

Create and edit pages with an explicit body format:

```bash
atl-conf page create --space DEV --title Notes \
  --body '<p>hi</p>' --body-format storage --site work --json='*'

atl-conf page edit 123456 --title "New title" \
  --body '<p>updated</p>' --body-format storage --site work --json='*'
```

Page delete moves content to trash by default. `--purge` permanently removes a
page that is already in the trash and requires `--yes`:

```bash
atl-conf page delete 123456 --site work
atl-conf page delete 123456 --site work --purge --yes
```

## Page Comments And Labels

```bash
atl-conf page comment list 123456 --site work --json='*'
atl-conf page comment view 999 --site work --json='*'
atl-conf page comment create 123456 --body '<p>Looks good.</p>' --body-format storage --site work --json='*'
atl-conf page comment edit 999 --body '<p>Updated.</p>' --body-format storage --site work --json='*'
atl-conf page comment delete 999 --site work --json='*'

atl-conf page label list 123456 --site work --json='*'
atl-conf page label add 123456 release-notes --site work --json='*'
atl-conf page label remove 123456 release-notes --site work --json='*'
```

## Blogposts, Attachments, Search

```bash
atl-conf blogpost list --space DEV --site work --limit 20 --json='*'
atl-conf blogpost view 123456 --site work --json='*'
atl-conf blogpost create --space DEV --title "Update" --body '<p>hi</p>' --body-format storage --site work --json='*'
atl-conf blogpost edit 123456 --title "Updated" --site work --json='*'

atl-conf attachment list 123456 --site work --json='*'
atl-conf attachment upload 123456 --file ./artifact.txt --site work --json='*'
atl-conf attachment download 999 --out ./artifact.txt --site work

atl-conf search cql 'type = page and space = DEV' --site work --limit 25 --json='*'
atl-conf search text notes --space DEV --type page --site work --json='*'
```

## Status And API Fallback

```bash
atl-conf status --site work --json='*'
atl-conf api /spaces --site work --jq '.results[] | {id, key, name}'
```

Prefer typed commands first. Use `api` for official Confluence REST endpoints
not wrapped by `atl-conf`.
