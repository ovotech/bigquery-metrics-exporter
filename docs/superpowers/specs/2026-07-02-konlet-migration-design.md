# Design: migrate off the deprecated container startup agent (konlet)

**Date:** 2026-07-02
**Status:** Draft — awaiting user review

## Background

The `terraform/gcp` module deploys `bqmetricsd` as a single VM in a Managed
Instance Group. Today the VM is a Container-Optimized OS (COS) image, and the
container is launched and supervised by Google's **container startup agent**
("konlet") based on a `gce-container-declaration` metadata entry written by the
`terraform-google-modules/container-vm` module.

Google has announced that the konlet-based container startup agent is being
deprecated. The suggested migration path is to launch the container from the
VM's startup script directly. This spec covers a drop-in replacement that keeps
the module's public interface unchanged.

## Goals

- Remove the dependency on konlet and on the `terraform-google-modules/container-vm`
  module.
- Preserve the module's public interface: existing variables, outputs, and
  behaviour so consumers do not need to edit their `.tf` files.
- Keep the diff as small as possible; no incidental refactoring.

## Non-goals

- Moving to a different runtime (Cloud Run, GKE) — explicitly out of scope.
- Adding systemd-based container supervision — considered, rejected in favour
  of simplicity given this application is low-criticality.
- Modernising `data "template_file"` to the `templatefile()` function.
- Changing logging, monitoring, healthcheck, IAM, firewall, or MIG behaviour.

## Approach

Extend `terraform/gcp/templates/startup.sh` so that in addition to writing the
config file it also runs `docker run -d --restart=always` for the metrics
daemon. Drop the `terraform-google-modules/container-vm` module and replace its
`source_image` lookup with a direct `data "google_compute_image"` against the
`cos-stable` family.

Docker's `--restart=always` policy provides restart-on-crash behaviour. We
accept a known tradeoff versus konlet/systemd: `--restart=always` engages only
after the container has run to completion at least once, so a container that
fails on very first launch (e.g. bad config, image-pull failure) is not
retried. Given this application is not critical and any first-launch failure
will be visible at `terraform apply` time (via `wait_for_instances = true`),
this is acceptable.

## Changes

### 1. `terraform/gcp/templates/startup.sh`

Replace the current 4-line script with:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Write configuration file for the bqmetrics daemon
mkdir -p "$(dirname "${config_path}")"
base64 -d > "${config_path}" <<< "${config_content}"

# Launch the daemon container. Host networking is used so the health check
# endpoint on port 8080 is reachable at the VM IP without explicit port
# publishing, matching the original konlet-managed behaviour.
docker rm -f bqmetricsd 2>/dev/null || true
docker run -d \
  --name=bqmetricsd \
  --restart=always \
  --network=host \
  -v "$(dirname "${config_path}")":"$(dirname "${config_path}")":ro \
  -e "LOG_LEVEL=${log_level}" \
  "${image}" \
  --config-file="${config_path}"
```

Rationale for the specific flags:

- `--network=host` matches konlet's default network mode on COS and keeps the
  MIG healthcheck reachable on port 8080 without needing `-p` publishing or
  changes to the existing firewall rule.
- `docker rm -f ... || true` before `docker run` makes the script idempotent
  across VM reboots (fresh MIG VMs won't have the container; a reboot would).
- No `--log-driver` flag is set, preserving the current behaviour of COS's
  json-file logs being picked up by fluent-bit when `google-logging-enabled=true`.
- `LOG_LEVEL` is passed via `-e`, matching the existing container spec's env
  block.
- `--config-file` is passed as a trailing arg to the container entrypoint
  (`bqmetricsd`), matching today's arg list.

### 2. `terraform/gcp/main.tf`

Delete the `module "container"` block. Replace references to it as follows.

Add a new data source for the COS image:

```hcl
data "google_compute_image" "cos" {
  family  = var.cos-image-family
  project = "cos-cloud"
}
```

In `google_compute_instance_template.bqmetricsd`:

- `disk.source_image` — change from `module.container.source_image` to
  `data.google_compute_image.cos.self_link`.
- `labels` — remove the entry keyed on `module.container.vm_container_label_key`
  (cosmetic label only, not consumed by any resource).
- `metadata` — remove the merge entry
  `{ (module.container.metadata_key) = module.container.metadata_value }`
  (this was the konlet declaration). Keep all other merge entries: the
  block-project-ssh-keys, enable-oslogin, google-monitoring-enabled, and
  google-logging-enabled entries all stay.

Extend the `data "template_file" "startup"` vars block with two additional
values so the new script can interpolate them:

```hcl
data "template_file" "startup" {
  template = file("${path.module}/templates/startup.sh")
  vars = {
    config_content = base64encode(jsonencode(local.config))
    config_path    = local.config_path
    image          = "${var.image-repository}:${var.image-tag}"
    log_level      = var.log-level
  }
}
```

### 3. `terraform/gcp/variables.tf`

Add one new optional variable with a safe default:

```hcl
variable "cos-image-family" {
  type        = string
  description = "The Container-Optimized OS image family to use for the VM boot disk"
  default     = "cos-stable"
}
```

No other variables change. `image-repository`, `image-tag`, `log-level`,
`custom-metrics`, `stackdriver-logging`, `stackdriver-monitoring`,
`enable-autohealing`, etc. all keep their existing names and semantics.

### 4. `terraform/gcp/output.tf`

No changes. `service-account-email` remains the only output.

### 5. `terraform/gcp/service-account.tf`

No changes. IAM bindings remain identical.

### 6. `terraform/gcp/module_version.txt`

Set to `1.5.0`. The file has drifted (currently `1.0.6`) while real releases
have continued via git tags (latest is `v1.4.0`). We correct the drift as part
of this release. Minor version bump reflects a substantive but
interface-preserving change.

### 7. `terraform/gcp/README.md`

- Update the example ref from `v1.2.2` to `v1.5.0`.
- Document the new `cos-image-family` variable.
- Add a short "Upgrading" note explaining that the first `terraform apply` after
  the bump will trigger one MIG rolling replacement and a COS image version step.

## Consumer impact

Consumers pin to a module ref like `?ref=v1.1.0`. Their existing `.tf` is
unchanged. On the first `terraform apply` after bumping the ref they will see:

- One `google_compute_instance_template` replacement (contents differ:
  `source_image`, `metadata`, `startup_script`). Because
  `create_before_destroy = true` and the MIG's update policy is
  `minimal_action = REPLACE, max_surge_fixed = 1`, this rolls the single VM
  with no service interruption beyond the brief startup window.
- The COS image ID will jump forward, because the old
  `terraform-google-modules/container-vm ~> 2.0` pin had likely fallen behind
  on COS milestones. Functionally fine; worth flagging in release notes.
- The `terraform-google-modules/container-vm` module and its child resources
  disappear from their state graph. Those are all data sources and locals; no
  real GCP resources are destroyed.

Consumers who depended on outputs from the container-vm sub-module would need
to update — but this module only exposes `service-account-email`, so no such
leak exists.

## Verification

Before tagging `v1.1.0`:

1. **`terraform plan` against a scratch project.** Expect: one instance-template
   replacement, one MIG rolling update, one new `data.google_compute_image`
   read, `terraform-google-modules/container-vm` module resources removed. No
   changes to service account, IAM, firewall, healthcheck, or outputs.
2. **`terraform apply`.** Confirm the MIG replaces the VM successfully under
   `wait_for_instances = true`.
3. **On the new VM, verify:**
   - `docker ps` shows `bqmetricsd` running with `--restart=always`.
   - `docker logs bqmetricsd` shows the daemon starting and querying BigQuery.
   - `curl http://<vm-ip>:8080/health` returns HTTP 200 (with autohealing on).
   - Metrics arrive in Datadog for a known table within one `metric-interval`.
4. **Kill-container test.** `docker kill bqmetricsd`, confirm `--restart=always`
   brings it back within seconds.
5. **Autohealing test (optional).** Blackhole port 8080 with an iptables rule
   and confirm the MIG replaces the VM.

## Accepted tradeoffs

- **No first-run supervision.** `--restart=always` does not cover the case
  where the container never successfully starts. Acceptable because
  first-launch failures surface at `terraform apply` time (thanks to
  `wait_for_instances = true`), and this application is not critical.
- **Deprecated `data "template_file"` retained.** The migration path from the
  deprecated hashicorp/template provider to the built-in `templatefile()`
  function is trivial, but out of scope for a "minimum change" migration.
