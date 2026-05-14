# Jira MVP

## API grounding

- Jira REST v3 intro: https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/
- Basic auth: https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/
- Atlassian account API tokens: https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/

## MVP command tree

```text
jira auth login|logout|status
jira api
jira resolve
jira browse
jira config
jira project list|view
jira issue list|view|create|edit|transition
jira issue comment list|view|create|edit|delete
jira search issues
jira status
```

## Design notes

- Do not fake universal close/reopen. Jira transitions are workflow-specific.
- Prefer JQL for issue search.
- Account IDs are the stable user identifiers; email/username lookup is privacy-limited.
- Boards/sprints come after core issue/project flows and product/license checks.
