# Token scope recipes

Recommended starter scope sets for Atlassian Cloud API tokens used with
`atl-jira`, `atl-conf`, and `atl-bb`. These are product-neutral recipes for
scripts and agents, not tenant-specific policy. Always tighten or broaden them
to the commands the token actually needs.

Atlassian's Jira and Confluence scope docs recommend staying below 50 scopes
and preferring classic scopes where possible. Bitbucket API-token scopes are
granular and do not imply matching read access from write or admin scopes, so
the Bitbucket recipe includes the read scopes explicitly.

Primary references:

- [Manage API tokens for your Atlassian account](https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/)
- [Jira product scopes](https://developer.atlassian.com/platform/forge/manifest-reference/scopes-product-jira/)
- [Jira Software product scopes](https://developer.atlassian.com/platform/forge/manifest-reference/scopes-product-jsw/)
- [Jira Service Management product scopes](https://developer.atlassian.com/platform/forge/manifest-reference/scopes-product-jsm/)
- [Confluence product scopes](https://developer.atlassian.com/platform/forge/manifest-reference/scopes-product-confluence/)
- [Bitbucket API token permissions](https://support.atlassian.com/bitbucket-cloud/docs/api-token-permissions/)
- [Bitbucket Cloud REST API scopes](https://developer.atlassian.com/cloud/bitbucket/rest/intro/#scopes)

Cached reflections of the Atlassian API-token picker, when captured, live under
[scope-snapshots/](scope-snapshots/). Treat those files as timestamped
observations, not standing recommendations.

## Jira

Compatibility-first core Jira set: 50 scopes total. It covers broad Jira,
Jira Software, and Jira Service Management issue/project workflows, prioritizes
viewing and ordinary creation/update, and omits delete scopes and `manage:org`.

### Classic scopes

1. `manage:jira-configuration`
2. `manage:jira-data-provider`
3. `manage:jira-project`
4. `manage:jira-webhook`
5. `manage:servicedesk-customer`
6. `read:account`
7. `read:jira-user`
8. `read:jira-work`
9. `read:me`
10. `read:servicedesk-request`
11. `write:jira-work`
12. `write:servicedesk-request`

### Granular scopes

1. `read:attachment:jira`
2. `read:board-scope:jira-software`
3. `read:comment:jira`
4. `read:epic:jira-software`
5. `read:field:jira`
6. `read:field.option:jira`
7. `read:group:jira`
8. `read:issue:jira`
9. `read:issue:jira-software`
10. `read:issue-details:jira`
11. `read:issue-field-values:jira`
12. `read:issue-link:jira`
13. `read:issue-link-type:jira`
14. `read:issue-meta:jira`
15. `read:issue-status:jira`
16. `read:issue-type:jira`
17. `read:issue-type-hierarchy:jira`
18. `read:issue-worklog:jira`
19. `read:issue.changelog:jira`
20. `read:issue.transition:jira`
21. `read:jql:jira`
22. `read:label:jira`
23. `read:permission:jira`
24. `read:priority:jira`
25. `read:project:jira`
26. `read:project.component:jira`
27. `read:project-version:jira`
28. `read:resolution:jira`
29. `read:sprint:jira-software`
30. `read:status:jira`
31. `read:user:jira`
32. `validate:jql:jira`
33. `write:attachment:jira`
34. `write:comment:jira`
35. `write:issue:jira`
36. `write:issue:jira-software`
37. `write:issue-link:jira`
38. `write:issue-worklog:jira`

### Tighten first

Good candidates to remove when the token only needs narrow issue workflows:

1. `read:group:jira`
2. `read:status:jira`
3. `read:user:jira`
4. `read:permission:jira`
5. `read:project.component:jira`

### Broaden only when needed

Common additions that are intentionally excluded from the core set:

- `manage:org`
- `read:email-address:jira`
- `read:board-scope.admin:jira-software`
- `write:board-scope:jira-software`
- `read:project-role:jira`
- `read:workflow:jira`
- `read:field-configuration:jira`

Also keep Assets/CMDB, development/build/deployment, feature-flag, and delete
scopes out of the default token unless the automation has a specific use for
them. For the cached picker reflection, see
[scope-snapshots/2026-06-09-jira-api-token-picker.md](scope-snapshots/2026-06-09-jira-api-token-picker.md).

## Confluence

Handcrafted broad Confluence set: 50 scopes total. It preserves broad
Confluence read/write coverage, including the official v2 APIs, without adding
whiteboard scopes or noisier admin-only scopes.

This scope set is designed to support both Confluence v1 and v2 API usage
through the Atlassian gateway URL shape. `atl-conf` is a mixed REST v1/v2
client; see [command-contract.md](command-contract.md#confluence-commands) for
the current endpoint split.

### Read scopes

1. `read:comment:confluence`
2. `read:blogpost:confluence`
3. `read:content.restriction:confluence`
4. `read:custom-content:confluence`
5. `read:database:confluence`
6. `read:email-address:confluence`
7. `read:embed:confluence`
8. `read:folder:confluence`
9. `read:hierarchical-content:confluence`
10. `read:inlinetask:confluence`
11. `read:label:confluence`
12. `read:page:confluence`
13. `read:permission:confluence`
14. `read:relation:confluence`
15. `read:space-details:confluence`
16. `read:space:confluence`
17. `read:space.permission:confluence`
18. `read:space.property:confluence`
19. `read:space.setting:confluence`
20. `read:task:confluence`
21. `read:template:confluence`
22. `read:user:confluence`
23. `read:user.property:confluence`
24. `read:group:confluence`
25. `read:attachment:confluence`
26. `read:content.property:confluence`
27. `read:content.permission:confluence`
28. `read:content.metadata:confluence`
29. `read:content:confluence`
30. `read:content-details:confluence`

### Write scopes

1. `write:attachment:confluence`
2. `write:blogpost:confluence`
3. `write:comment:confluence`
4. `write:content:confluence`
5. `write:content.property:confluence`
6. `write:content.restriction:confluence`
7. `write:custom-content:confluence`
8. `write:database:confluence`
9. `write:embed:confluence`
10. `write:folder:confluence`
11. `write:inlinetask:confluence`
12. `write:label:confluence`
13. `write:page:confluence`
14. `write:relation:confluence`
15. `write:space:confluence`
16. `write:space.permission:confluence`
17. `write:space.property:confluence`
18. `write:space.setting:confluence`
19. `write:task:confluence`
20. `write:template:confluence`

### Tighten first

Highest-risk enabled scope:

- `write:space.permission:confluence` can change space-level permissions. Keep
  it only when automation is expected to manage space ACLs.

Sensitive read scope:

- `read:email-address:confluence` can expose user email addresses regardless of
  profile visibility. Keep it only when email data is required.

Good candidates to remove when tightening:

1. `read:relation:confluence`
2. `read:user.property:confluence`
3. `read:group:confluence`
4. `write:space.permission:confluence`
5. `write:relation:confluence`

### Broaden only when needed

These categories are intentionally left off the final token:

- Whiteboard scopes
- App-data scopes
- Audit-log scopes
- Global configuration scopes
- Group-write scopes

Those scopes are either not used by the current Confluence workflows or are
better handled by a separate break-glass token. For the cached picker
reflection, see
[scope-snapshots/2026-06-09-confluence-api-token-picker.md](scope-snapshots/2026-06-09-confluence-api-token-picker.md).

## Bitbucket

Recommended broad `atl-bb` token: 15 scopes total. Bitbucket Cloud API tokens
authenticate with Basic auth against `https://api.bitbucket.org/2.0`, using the
Atlassian account email as the username. They do not use the Jira/Confluence
`api.atlassian.com/ex/<product>/<cloudId>` gateway.

This set covers `atl-bb` status, workspace/project/repository reads, repository
create, branch/tag/source/commit reads and writes, pull request review/write
flows, issue read/write flows, pipeline read/run/stop/log flows, and deployment
and environment reads. It intentionally excludes repository deletion, workspace
administration, account mutation, pipeline variables, runners, webhooks,
snippets, SSH/GPG key management, and permission CRUD.

### Core scopes

1. `admin:project:bitbucket`
2. `admin:repository:bitbucket`
3. `read:account`
4. `read:issue:bitbucket`
5. `read:me`
6. `read:pipeline:bitbucket`
7. `read:project:bitbucket`
8. `read:pullrequest:bitbucket`
9. `read:repository:bitbucket`
10. `read:user:bitbucket`
11. `read:workspace:bitbucket`
12. `write:issue:bitbucket`
13. `write:pipeline:bitbucket`
14. `write:pullrequest:bitbucket`
15. `write:repository:bitbucket`

### Optional destructive scope

Add only for automation that must run `atl-bb repo delete --yes`:

- `delete:repository:bitbucket`

`admin:project:bitbucket` also permits project deletion because Bitbucket does
not expose a separate project delete scope. Use a narrower token without this
scope for read/write repository and pull request automation that does not need
project create/delete.

### Tighten first

Good candidates to remove when the token only needs common repository and pull
request workflows:

1. `admin:project:bitbucket`
2. `admin:repository:bitbucket`
3. `write:pipeline:bitbucket`
4. `write:issue:bitbucket`
5. `write:repository:bitbucket`

Do not remove the matching read scopes just because a write/admin scope is
present. Bitbucket's API-token scopes are explicit; write/admin scopes do not
automatically include read scopes.

### Broaden only when needed

Current picker scopes outside this starter set are either for surfaces `atl-bb`
does not manage by default, or for admin/destructive workflows that are better
isolated in a separate break-glass token. See
[scope-snapshots/2026-06-09-bitbucket-api-token-picker.md](scope-snapshots/2026-06-09-bitbucket-api-token-picker.md)
for the cached picker reflection that informed this section.
