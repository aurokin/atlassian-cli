# Jira Reference

Use `atl-jira` for Jira Cloud or Jira Data Center work.

## Common Commands

```bash
atl-jira project list --site work --json='*'
atl-jira project view DEV --site work --json='*'

atl-jira issue view PROJ-123 --site work --json='*'
atl-jira issue view PROJ-123 --site work --fields summary,status,assignee --jq '.fields'
atl-jira issue list --project PROJ --site work --limit 50 --json='*'
atl-jira search issues 'project = PROJ ORDER BY updated DESC' --site work --limit 50 --json='*'
```

## Issue Mutations

```bash
atl-jira issue create --project PROJ --type Task --summary "Follow up" --site work --json='*'
atl-jira issue edit PROJ-123 --summary "Updated summary" --site work --json='*'
atl-jira issue transition PROJ-123 --to Done --site work --json='*'
atl-jira issue assign PROJ-123 @me --site work --json='*'
atl-jira issue assign PROJ-123 - --site work --json='*'
```

Use `--field name=value` for fields that do not have first-class flags. Field
names and JSON shapes follow Jira's API.

## Comments, Worklogs, Links, Watchers

```bash
atl-jira issue comment list PROJ-123 --site work --order desc --limit 20 --json='*'
atl-jira issue comment create PROJ-123 --body "Investigating." --site work --json='*'
atl-jira issue comment edit PROJ-123 10001 --body "Updated." --site work --json='*'
atl-jira issue comment delete PROJ-123 10001 --site work --json='*'

atl-jira issue worklog list PROJ-123 --site work --json='*'
atl-jira issue worklog add PROJ-123 --time 1h --comment "Implementation" --site work --json='*'

atl-jira issue link PROJ-123 PROJ-456 --type Blocks --site work --json='*'
atl-jira issue link types --site work --json='*'

atl-jira issue watchers PROJ-123 --site work --json='*'
atl-jira issue watch PROJ-123 --site work --json='*'
atl-jira issue unwatch PROJ-123 --site work --json='*'
```

## Attachments And Fields

```bash
atl-jira issue attachment list PROJ-123 --site work --json='*'
atl-jira issue attachment add PROJ-123 --file ./artifact.txt --site work --json='*'
atl-jira issue attachment download 10001 --out ./artifact.txt --site work
atl-jira field list --site work --json='*'
```

Use `field list` before guessing custom-field ids.

## Status And API Fallback

```bash
atl-jira status --site work --json='*'
atl-jira api /myself --site work --json='*'
atl-jira api /issue/PROJ-123 --site work --jq '.fields.summary'
```

Prefer typed commands first. Use `api` for official Jira REST endpoints not
wrapped by `atl-jira`.
