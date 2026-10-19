# SonarQube Community Build on the LAN

Community Build analyzes the imported project's main branch only. This integration
**does not run on merge-request, feature-branch, or tag pipelines** and does not
provide MR decoration. Native branch/MR analysis requires a supported commercial
edition. Do not force MR code into this project's main analysis or add
`sonar.branch.name` / `sonar.pullrequest.*` properties as a workaround.

## 1. Check runner connectivity

The project runners currently have `linux` and `neucleus.local` tags. The Sonar
job requires both, so it stays on the LAN runners even if other runners are added.
The pinned scanner image supports their current Linux amd64 architecture.

SonarQube's URL is `http://192.168.1.100:9000`. Job containers must reach that
address: container `localhost` is not the host, and `.local` mDNS need not work
inside Docker. On the runner host, check from a container:

```sh
docker run --rm curlimages/curl:8.17.0 \
  --fail --show-error --connect-timeout 5 --max-time 15 \
  http://192.168.1.100:9000/api/system/status
```

Use the runner's configured Docker network if it differs from the default bridge
(`--network NETWORK`). Expect status `UP`; HTTP 401/403 establishes connectivity
but means the server requires authentication. Timeout/refusal means check the
Sonar service/container port publication, listening interface and host firewall
before enabling CI. Allow the runner Docker subnet; do not expose port 9000 to
the internet or disable authentication. Rootless/custom Docker network isolation
may also require host configuration. This connectivity check submits no analysis
and needs no token. A successful host-browser check alone does not prove job
container connectivity.

HTTP carries the analysis token and source data without transport encryption.
Use it only on a trusted LAN/Docker network. Prefer HTTPS with a certificate trusted
by the scanner if the network is shared; do not bypass certificate verification.
Keep runners restricted to trusted project contributors: omitting the Sonar job
from MRs does not isolate arbitrary CI code from other LAN services.

## 2. Configure SonarQube and GitLab variables

The GitLab import already exists; do not create another Sonar project. In its
**Project Information**, copy the exact project key. Ensure the Sonar main branch
matches GitLab's default branch (`main`) before the first analysis.

In SonarQube **My Account → Security**, generate an expiring **project analysis
token** for this project using an account with Execute Analysis permission.
Do not use the GitLab personal access token configured for project import.
Do not paste either token into chat, the repository, commands, or job logs.

In GitLab **Settings → CI/CD → Variables**, add ordinary Variable-type values
(not File type), with environment scope `*`:

| Variable | Value | Protection |
| --- | --- | --- |
| `SONAR_HOST_URL` | `http://192.168.1.100:9000` | Protected |
| `SONAR_PROJECT_KEY` | Exact key of the imported Sonar project | Protected |
| `SONAR_TOKEN` | Project analysis token | Protected, Masked, and Hidden if available |
| `SONAR_ENABLED` | `1` after the other settings are ready | Protected |

`main` is already protected. Keep these variables protected so feature branches
and ordinary MR pipelines cannot receive the token. Do not enable debug tracing
for jobs containing credentials. Review changes to CI on protected branches.
Rotate the token before expiry and revoke it when no longer used.

Review the project's assigned quality gate and new-code definition in SonarQube.
The job waits up to 300 seconds for its quality gate and fails on a rejected gate,
scanner/server error, missing configuration, or timeout. The gate is not advisory;
there is no `allow_failure`. A failing main pipeline does not undo its merge.
Community Build cannot provide this gate before merge with native MR analysis.

## 3. Merge and run

1. Merge the integration MR after its normal CI checks pass. Its MR pipeline
   intentionally performs no Sonar upload, even if the variables exist.
2. With the variables configured, the default-branch pipeline runs `sonarqube`
   after the verification and build stages succeed. Alternatively, use GitLab
   **Build → Pipelines → New pipeline** on `main` after configuring the variables.
3. Confirm the job used a LAN runner and the imported Sonar project shows the
   expected commit/analysis and quality-gate result. Inspect coverage import
   warnings; do not treat a successful CI lint or an omitted job as a live scan.

Unset `SONAR_ENABLED` or set it to `0` to stop scheduling future scans. When it is
`1`, missing settings fail explicitly rather than silently omitting analysis.
No job is scheduled on MRs, other branches or tags. The full Git history is fetched
for blame/new-code analysis. The scanner image runs as UID 1000; if checkout or
cache writes fail, fix runner workspace/cache permissions for that UID rather
than running the scanner privileged. Keep analysis on the latest main commit; do not rerun
old main pipelines, which could replace the dashboard with an older analysis.
A resource group serializes scans and a running scan is non-interruptible so its
quality-gate wait is not abandoned by automatic pipeline cancellation.

The scanner consumes `coverage/coverage.out` from the existing `verify` artifact;
it does not rerun tests or generate/convert coverage. Production and test Go files
are classified separately, and test fixtures are not production source. Sonar's
line/new-code coverage can differ from Make's aggregate statement coverage and
platform scope. The existing **90% Make coverage gate is unchanged**, as are race,
recovery, D-Bus, real-mpv and build jobs. No native macOS coverage is claimed from
this Linux report. Scanner/analyzer downloads require the appropriate outbound
connectivity; use a Sonar version with Go analysis compatible with this project's
Go version. Update the pinned scanner tag/digest deliberately when upgrading.

## References

- [Community Build GitLab integration](https://docs.sonarsource.com/sonarqube-community-build/analyzing-source-code/ci-integration/gitlab-integration/)
- [SonarScanner CLI](https://docs.sonarsource.com/sonarqube-community-build/analyzing-source-code/scanners/sonarscanner)
- [Go coverage import](https://docs.sonarsource.com/sonarqube-community-build/analyzing-source-code/test-coverage/go-test-coverage/)
