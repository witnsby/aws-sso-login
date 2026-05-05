---
name: cicd-github-actions
description: Domain skill for CI/CD pipelines using GitHub Actions. Covers workflow structure, job design, security hardening, caching, and reusable workflow patterns. Use when working on .github/workflows/ files. Applies to **/.github/workflows/*.yml, **/.github/actions/**/*.yml files.
---

# CI/CD — GitHub Actions

## Domain
CI/CD pipelines under `.github/workflows/`.

## Toolchain
- **Platform**: GitHub Actions
- **Config format**: YAML
- **CLI**: `gh` (GitHub CLI)

## Quality Gate Commands

```bash
# Validate workflow syntax (requires actionlint)
actionlint .github/workflows/*.yml

# Or basic YAML validation
yamllint .github/workflows/
```

## Core Philosophy: Prefer Marketplace Actions Over Shell Scripts

**Always reach for a marketplace action before writing a `run:` shell step.** Shell steps in CI are harder to test, harder to read, error-prone, and bypass the ecosystem of well-tested, auto-masked, and maintained actions.

### Decision rule

```
Does a maintained marketplace action exist for this task?
  YES → Use it (SHA-pinned). Only add a run: step if bridging is unavoidable.
  NO  → Write a minimal, focused run: step. Keep it under 5 lines.
```

### What this looks like in practice

| Task | Shell approach (avoid) | Marketplace action (prefer) |
|------|------------------------|-----------------------------|
| Fetch AWS secret | `aws secretsmanager get-secret-value \| jq` | `abhilash1in/aws-secrets-manager-action` |
| Docker login | `echo $TOKEN \| docker login` | `docker/login-action` |
| Build & push image | `docker build && docker push` | `docker/build-push-action` |
| Setup language runtime | `curl \| bash \| export PATH` | `actions/setup-node`, `actions/setup-python` |
| Upload/download artifacts | `cp`, `tar`, `scp` | `actions/upload-artifact`, `actions/download-artifact` |
| Git operations | `git tag && git push` | `actions/github-script` or dedicated action |
| Slack/Teams notify | `curl -X POST webhook` | dedicated notification action |
| Terraform plan/apply | raw `terraform` CLI steps | `hashicorp/setup-terraform` + focused `run:` |

### When `run:` is acceptable
- Glue between two actions (bridge pattern) — keep it under 5 lines
- Truly bespoke logic with no action equivalent
- Simple one-liners (`echo`, `cat`, arithmetic)

### When `run:` is NOT acceptable
- Fetching secrets manually (`aws secretsmanager`, `vault`)
- Docker auth (`docker login`)
- Any task where a well-maintained action exists

## Critical Patterns

1. **Pin actions by SHA, not tag**
   ```yaml
   # Correct — SHA-pinned with version comment
   - uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1

   # FORBIDDEN — tag only
   - uses: actions/checkout@v4
   ```
   Applies to all actions including marketplace ones. Dependabot understands this format and updates both SHA and comment automatically.

2. **Restrict permissions to minimum required**
   ```yaml
   permissions:
     contents: read
     pull-requests: write
   ```
   Never use `permissions: write-all`. Set at workflow level; override per job if needed.

3. **Never expose secrets in logs**
   - Use `${{ secrets.NAME }}`, never echo secrets
   - Marketplace actions handle masking internally — prefer them over manual `::add-mask::`
   - Only use `echo "::add-mask::$VALUE"` when writing a `run:` step that handles sensitive values directly

4. **Cache dependencies for speed**
   ```yaml
   - uses: actions/setup-node@...
     with:
       node-version: '22'
       cache: 'pnpm'

   - uses: actions/setup-python@...
     with:
       python-version: '3.13'
       cache: 'pip'
   ```

5. **Use `concurrency` to prevent duplicate runs**
   ```yaml
   concurrency:
     group: ${{ github.workflow }}-${{ github.ref }}
     cancel-in-progress: true
   ```

6. **Conditional execution for monorepos**
   ```yaml
   on:
     pull_request:
       paths:
         - 'backend/**'
         - '.github/workflows/backend-*.yml'
   ```

## Approved Marketplace Actions (SHA-Pinned Catalog)

Preferred actions for common tasks. Always verify the latest SHA before use.

| Action | Version | Use for |
|--------|---------|---------|
| `actions/checkout` | v4 | Repo checkout |
| `actions/setup-node` | v4 | Node.js runtime |
| `actions/setup-python` | v5 | Python runtime |
| `actions/upload-artifact` | v4 | Artifact persistence |
| `actions/download-artifact` | v4 | Artifact retrieval |
| `aws-actions/configure-aws-credentials` | v4 | OIDC-based AWS auth |
| `aws-actions/amazon-ecr-login` | v2 | ECR registry auth |
| `aws-actions/aws-secretsmanager-get-secrets` | v2.0.10 | AWS Secrets Manager fetch |
| `docker/login-action` | v3 | Docker registry auth |
| `docker/build-push-action` | v6 | Build and push images |
| `docker/metadata-action` | v5 | Image tag generation |
| `hashicorp/setup-terraform` | v3 | Terraform CLI setup |
| `int128/kaniko-action` | v1 | Kaniko builds |

## AWS Secrets Manager Pattern

Use `aws-actions/aws-secretsmanager-get-secrets` (official AWS-maintained action) — never raw `aws` CLI.

```yaml
- uses: aws-actions/aws-secretsmanager-get-secrets@a9a7eb4e2f2871d30dc5b892576fde60a2ecc802 # v2.0.10
  with:
    secret-ids: |
      MY_ALIAS, /path/to/secret
    parse-json-secrets: true
```

Key behaviors:
- `parse-json-secrets: true` flattens JSON keys — `{"username":"x","token":"y"}` → `MY_ALIAS_USERNAME` and `MY_ALIAS_TOKEN`
- All injected values are **automatically masked** — no manual `::add-mask::` needed
- **Alias syntax** (`ALIAS, secret-id`) pins the env var name regardless of the secret path — critical when the path contains a dynamic runtime prefix
- `${{ env.* }}` context **is** available in `with:` fields, so dynamic paths work directly:

```yaml
- uses: aws-actions/aws-secretsmanager-get-secrets@a9a7eb4e2f2871d30dc5b892576fde60a2ecc802 # v2.0.10
  with:
    secret-ids: |
      DOCKERHUB, /${{ env.RESOURCE_PREFIX }}/dockerhub
    parse-json-secrets: true
- uses: docker/login-action@c94ce9fb468520275223c153574b00df6fe4bcc9 # v3.7.0
  with:
    username: ${{ env.DOCKERHUB_USERNAME }}
    password: ${{ env.DOCKERHUB_TOKEN }}
```

No `run:` bridge step needed. The alias gives a static, predictable env var name (`DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`) even when the secret path is environment-specific.

**IAM requirements:** `secretsmanager:GetSecretValue` on the target secret. Wildcard prefix patterns (e.g., `dev*`) additionally require `secretsmanager:ListSecrets`; direct paths and aliases do not.

## Workflow Structure

```yaml
name: descriptive-name

on:
  pull_request:
    branches: [main]
    paths: [...]
  push:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@SHA # vX.Y.Z
      # ...

  test:
    needs: lint
    runs-on: ubuntu-latest
    steps:
      # ...
```

## Reusable Workflows

```yaml
# .github/workflows/reusable-test.yml
on:
  workflow_call:
    inputs:
      service:
        required: true
        type: string

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@SHA
      - run: cd ${{ inputs.service }} && make test
```

Called from another workflow:
```yaml
jobs:
  test-auth:
    uses: ./.github/workflows/reusable-test.yml
    with:
      service: backend/auth-service
```

## Job Design Principles

- **Fail fast**: Put lint/format checks before expensive test jobs
- **Parallel where possible**: Independent jobs should not use `needs:`
- **Matrix for variants**: Use `strategy.matrix` for multi-version/multi-service testing
- **Artifacts for handoff**: Use `actions/upload-artifact` / `actions/download-artifact` between jobs
- **Timeouts**: Always set `timeout-minutes` to prevent hung jobs

## Common Pitfalls

| Pitfall | Fix |
|---------|-----|
| Shell script doing what an action could do | Check marketplace first — if an action exists, use it |
| Actions pinned by tag, not SHA | Always use full 40-char SHA with version comment |
| Workflow runs on all pushes | Add `paths:` filter for monorepo |
| Slow CI | Add dependency caching, parallelize jobs |
| Flaky tests | Add retry with `nick-fields/retry` action |
| Secret in fork PRs | Use `pull_request_target` carefully or restrict to `push` |
| Large checkout | Use `fetch-depth: 0` only when needed (e.g., changelog generation) |
| Manual secret masking | Use marketplace actions that handle masking internally |

## Related Skills
- `docker-containers` — building and pushing container images
- `gitops` — CI feeds into GitOps deployment flow
- `auto-qa` — quality gates run in CI
- `policy-as-code` — conftest validation in pipeline
- `finops` — infracost in PR comments
- `terraform-infra` — Terraform plan/apply in CI
- `ecs-fargate` — ECS Fargate task definitions, services, autoscaling, deployments
