> **Historical / archived.** This document is a completed planning or design artifact, kept for project history. It is **not** maintained and may not reflect the current code. For the implemented behavior see [`docs/command-contract.md`](../command-contract.md) and [`docs/shared-architecture.md`](../shared-architecture.md).

# Jira MVP

## API grounding

- Jira REST v3 intro: https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/
- Basic auth: https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/
- Atlassian account API tokens: https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/

## MVP command tree

```text
atl-jira auth login|logout|status
atl-jira api
atl-jira resolve
atl-jira browse
atl-jira config
atl-jira project list|view
atl-jira issue list|view|create|edit|transition
atl-jira issue assign|watch|unwatch|watchers
atl-jira issue link [<inward> <outward> --type <link-type>] | types
atl-jira issue worklog list|add
atl-jira issue comment list|view|create|edit|delete
atl-jira search issues
atl-jira status
```

## Design notes

- Do not fake universal close/reopen. Jira transitions are workflow-specific.
- Prefer JQL for issue search.
- Account IDs are the stable user identifiers; email/username lookup is privacy-limited.
- Boards/sprints come after core issue/project flows and product/license checks.
- `issue assign -` unassigns; setting a project's default assignee is out of scope.
- `issue link <inward> <outward> --type` matches Jira's API field names verbatim.
- `issue worklog add --time` is passed through verbatim; the CLI never parses or
  converts duration strings.
