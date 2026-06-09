# Bitbucket Cloud Reference

Use `atl-bb` for Bitbucket Cloud. It does not target Bitbucket Server/Data
Center.

## Contents

- Repository targeting
- Repositories, workspaces, and projects
- Pull requests
- Pipelines, issues, and source
- Branches, tags, deployments, and search
- Status and API fallback

## Repository Targeting

Prefer explicit repositories:

```bash
atl-bb repo view workspace/repo --site work --json='*'
atl-bb repo view --repo workspace/repo --site work --json='*'
```

Use `--workspace <slug>` only to disambiguate a bare repository name. Some
commands can infer the repository from a git checkout; explicit `--repo` is
still better for agents.

## Repositories, Workspaces, Projects

```bash
atl-bb repo list workspace --site work --limit 50 --json='*'
atl-bb repo create workspace/repo --site work --description "New repo" --private --json='*'
atl-bb repo delete workspace/repo --site work --yes

atl-bb workspace view workspace --site work --json='*'
atl-bb project list workspace --site work --json='*'
atl-bb project view DEV --workspace workspace --site work --json='*'
atl-bb project create DEV --workspace workspace --name "Developer Tools" --site work --json='*'
atl-bb project delete DEV --workspace workspace --site work --yes
```

Repository and project deletion require `--yes`.

## Pull Requests

```bash
atl-bb pr list --repo workspace/repo --site work --state OPEN --limit 50 --json='*'
atl-bb pr view 7 --repo workspace/repo --site work --json='*'
atl-bb pr create --repo workspace/repo --site work --title "Change" --source feature --destination main --json='*'
atl-bb pr approve 7 --repo workspace/repo --site work --json='*'
atl-bb pr unapprove 7 --repo workspace/repo --site work --json='*'
atl-bb pr decline 7 --repo workspace/repo --site work --json='*'
atl-bb pr merge 7 --repo workspace/repo --site work --json='*'
atl-bb pr diff 7 --repo workspace/repo --site work
atl-bb pr comments list 7 --repo workspace/repo --site work --json='*'
atl-bb pr comments add 7 --repo workspace/repo --site work --body "Looks good." --json='*'
```

`pr diff` is raw text, so `--json` and `--jq` do not apply.

## Pipelines, Issues, Source

```bash
atl-bb pipeline list --repo workspace/repo --site work --limit 20 --json='*'
atl-bb pipeline view 1 --repo workspace/repo --site work --json='*'
atl-bb pipeline run --repo workspace/repo --site work --ref main --ref-type branch --json='*'
atl-bb pipeline stop 1 --repo workspace/repo --site work --json='*'
atl-bb pipeline steps 1 --repo workspace/repo --site work --json='*'
atl-bb pipeline log 1 "{step-uuid}" --repo workspace/repo --site work

atl-bb issue list --repo workspace/repo --site work --state open --json='*'
atl-bb issue view 12 --repo workspace/repo --site work --json='*'
atl-bb issue create --repo workspace/repo --site work --title "Bug" --kind bug --json='*'
atl-bb issue update 12 --repo workspace/repo --site work --state resolved --json='*'

atl-bb src --repo workspace/repo --site work --ref main --json='*'
atl-bb file README.md --repo workspace/repo --site work --ref main
```

File contents and pipeline logs are raw output; do not expect JSON there.

## Branches, Tags, Deployments, Search

```bash
atl-bb branch list --repo workspace/repo --site work --json='*'
atl-bb branch create --repo workspace/repo --site work --name feature --target main --json='*'
atl-bb branch delete feature --repo workspace/repo --site work

atl-bb tag list --repo workspace/repo --site work --json='*'
atl-bb tag create --repo workspace/repo --site work --name v1.0.0 --target abc123 --json='*'
atl-bb tag delete v1.0.0 --repo workspace/repo --site work

atl-bb deployment list --repo workspace/repo --site work --json='*'
atl-bb environment list --repo workspace/repo --site work --json='*'

atl-bb search repos cli --workspace workspace --site work --json='*'
atl-bb search prs bugfix --repo workspace/repo --site work --json='*'
atl-bb search issues crash --repo workspace/repo --site work --json='*'
```

## Status And API Fallback

```bash
atl-bb status --site work --json='*'
atl-bb api /user --site work --json='*'
atl-bb api /repositories/workspace/repo/pullrequests --site work --jq '.values[] | {id, title, state}'
```

Prefer typed commands first. Use `api` for official Bitbucket Cloud REST
endpoints not wrapped by `atl-bb`.
