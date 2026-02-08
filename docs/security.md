# Security

This document outlines security best practices for deploying and operating `mcp-sandboxd`. Given that `mcp-sandboxd` executes arbitrary code in isolated sandbox environments, careful consideration of security is paramount.

**Related docs**

- [Architecture](architecture.md)
- [Configuration](configuration.md)
- [Docker](docker.md)
- [Kubernetes](kubernetes.md)
- [Observability](observability.md)
- [Security](security.md)

## 1. Ingress Traffic

The `mcp-sandboxd` server exposes an HTTP JSON-RPC endpoint (default `POST /mcp`) and a metrics endpoint (default `GET /metrics`). These endpoints allow users to interact with sandboxes and gather operational data. Unauthorized access or abuse can lead to arbitrary code execution, denial-of-service, or information disclosure.

### 1.1. Authorization and Authentication

Implement robust authorization and authentication for the `/mcp` and metrics endpoints. **Never expose these endpoints directly to untrusted clients without protection.**

**Recommended**

- **Ingress Proxy** Front `mcp-sandboxd` with an Ingress Proxy that enforces authentication before forwarding requests.
- **Service Mesh Authorization** If using a service mesh, leverage its authorization policies to control which identities can access the `mcp-sandboxd` service.
- **Network-level Access Control** Restrict network access to the `mcp-sandboxd` service by allowing connections only from trusted sources (e.g., specific IP ranges, internal cluster services).

### 1.2. Rate Limiting

Implement rate limiting on the `/mcp` and metrics endpoints to prevent Denial-of-Service (DoS) attacks and enumeration of random sandbox identifiers.

**Strategies**

- **Ingress Proxy** Configure your Ingress Controller to apply rate limits based on source IP, authenticated user, or HTTP headers.
- **Application-level** While `mcp-sandboxd` itself does not have built-in comprehensive rate limiting for individual users/sessions, deployers can implement middleware to track and limit requests based on `Mcp-Session-Id` header or other client identifiers.

## 2. Resource Limiting

**Server-side Limits**

`mcp-sandboxd` includes some internal limits to prevent abuse, configurable via environment variables (see [Configuration](configuration.md)):

- [`MAX_RUNS`](configuration.md#MAX_RUNS): Limits the total number of concurrent runs.
- [`DEFAULT_COMMAND_TIMEOUT_MS`](configuration.md#DEFAULT_COMMAND_TIMEOUT_MS): Prevents individual commands from running indefinitely.
- [`DEFAULT_STDOUT_MAX_BYTES`](configuration.md#DEFAULT_STDOUT_MAX_BYTES), [`DEFAULT_STDERR_MAX_BYTES`](configuration.md#DEFAULT_STDERR_MAX_BYTES): Limits the size of output returned, preventing memory exhaustion from large sandbox outputs.
- [`DEFAULT_TTL_SECONDS`](configuration.md#DEFAULT_TTL_SECONDS), [`MAX_TTL_SECONDS`](configuration.md#MAX_TTL_SECONDS): Control how long sandboxes persist, preventing indefinite resource consumption.

Limit the resources consumed by sandbox containers and Pods to prevent DoS attacks against the underlying infrastructure and ensure fair resource sharing.

### 2.1. Kubernetes Resource Quotas and Limit Ranges

For Kubernetes deployments, use [`LimitRange`](https://kubernetes.io/docs/concepts/policy/limit-range/) and [`ResourceQuota`](https://kubernetes.io/docs/concepts/policy/resource-quotas/) in the sandbox namespace to enforce default resource requests/limits and cap total resource usage.

#### `LimitRange`

Enforces defaults for containers

```yaml
apiVersion: v1
kind: LimitRange
metadata:
  name: sandbox-limits
  namespace: mcp-sandboxd-sandboxes
spec:
  limits:
    - default:
        cpu: 500m
        memory: 512Mi
      defaultRequest:
        cpu: 100m
        memory: 256Mi
      type: Container
    - type: Pod
      max:
        cpu: 1
        memory: 1Gi
```

#### `ResourceQuota`

Caps total namespace resources

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: sandbox-quota
  namespace: mcp-sandboxd-sandboxes
spec:
  hard:
    pods: "20"
    requests.cpu: "5"
    requests.memory: "10Gi"
    limits.cpu: "10"
    limits.memory: "20Gi"
    ephemeral-storage: "50Gi"
```

## 3. Container Capabilities and Privilege Controls

[Linux Capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html) allow granular control over root-like privileges. `mcp-sandboxd` configures sandbox containers with a minimal set of capabilities by default.

**Recommended**

- [`SANDBOX_CAP_DROP`](configuration.md#SANDBOX_CAP_DROP) (default `ALL`): Drops all capabilities by default, enforcing a secure baseline.
- [`SANDBOX_CAP_ADD`](configuration.md#SANDBOX_CAP_ADD) (default `SETUID,SETGID,CHOWN,FOWNER,DAC_OVERRIDE`): Adds back only essential capabilities required for basic file operations and user management within the sandbox.
- [`SANDBOX_NO_NEW_PRIVILEGES`](configuration.md#SANDBOX_NO_NEW_PRIVILEGES) (default `true`): Prevents privilege escalation within the container. Kubernetes Pods automatically set `allowPrivilegeEscalation: false`.
- Kubernetes Pods also enforce `seccompProfile: RuntimeDefault` for enhanced syscall filtering.

**Principle of Least Privilege**

Only add capabilities ([`SANDBOX_CAP_ADD`](configuration.md#SANDBOX_CAP_ADD)) if they are strictly necessary for your sandbox's workload. Avoid adding dangerous capabilities like:
- [`NET_RAW`](https://man7.org/linux/man-pages/man7/capabilities.7.html): Allows creation of raw network sockets, enabling custom packet crafting, advanced scanning, and some forms of network bypass (e.g., `ping`). If required, use it with extreme caution and strong compensating network controls.
- [`NET_ADMIN`](https://man7.org/linux/man-pages/man7/capabilities.7.html): Allows network configuration, including setting up network interfaces, firewalls, and routing rules.
- [`SYS_ADMIN`](https://man7.org/linux/man-pages/man7/capabilities.7.html): Grants a wide range of administrative system operations.
- [`DAC_READ_SEARCH`](https://man7.org/linux/man-pages/man7/capabilities.7.html), [`DAC_OVERRIDE`](https://man7.org/linux/man-pages/man7/capabilities.7.html): Allows bypassing file read/search permissions, potentially allowing access to sensitive files.
- [`CHOWN`](https://man7.org/linux/man-pages/man7/capabilities.7.html), [`FOWNER`](https://man7.org/linux/man-pages/man7/capabilities.7.html): Allow arbitrary file ownership changes, potentially leading to unauthorized access.

**Kyverno Policy**

Consider using a [Kyverno](https://kyverno.io/) policy to enforce these `securityContext` settings at admission time, ensuring consistency across all sandbox Pods.

#### `Policy`

```yaml
apiVersion: kyverno.io/v1
kind: Policy
metadata:
  name: mcp-sandboxd-force-sandbox-caps
spec:
  validationFailureAction: Enforce
  background: true
  rules:
    - name: force-sandbox-container-securitycontext
      match:
        any:
          - resources:
              kinds:
                - Pod
              selector:
                matchLabels:
                  mcp-sandboxd.jeliasson.dev/role: sandbox
      mutate:
        patchStrategicMerge:
          spec:
            containers:
              - (name): sandbox
                securityContext:
                  allowPrivilegeEscalation: false
                  seccompProfile:
                    type: RuntimeDefault
                  capabilities:
                    drop:
                      - ALL
                    add:
                      - SETUID
                      - SETGID
                      - CHOWN
                      - FOWNER
                      - DAC_OVERRIDE
                      - NET_RAW
```

## 4. Egress Traffic

Restrict outbound network access from sandbox containers to only what is strictly necessary. This limits the blast radius of compromised sandboxes, preventing them from attacking internal services or exfiltrating data to arbitrary external destinations.

**Recommended**

- **Default-Deny Network Policy** Implement a default-deny egress [`NetworkPolicy`](https://kubernetes.io/docs/concepts/services-networking/network-policies/) for the sandbox namespace that explicitly allows only required outbound connections (e.g., to DNS servers, required external APIs, package repositories).

#### `NetworkPolicy`

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: sandbox-egress-restrict
  namespace: mcp-sandboxd-sandboxes
spec:
  podSelector: {}
  policyTypes:
    - Egress
  egress:
    - to:
        - ipBlock:
            cidr: 9.9.9.9/32
            except: []
        - ipBlock:
            cidr: 1.1.1.1/32
            except: []
      ports:
        - protocol: UDP
          port: 53
```

**Crucial Blocks**

- **Cloud Metadata Service** Explicitly block access to cloud provider metadata service IPs (e.g., `169.254.169.254/32` for AWS, GCP, Azure). This prevents a compromised sandbox from stealing temporary credentials.
- **RFC1918 / Private IP Ranges** Ensure your policy blocks egress to internal network ranges (e.g., `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`) to prevent lateral movement within your VPC or cluster.
- **Kubernetes API** Block access to the Kubernetes API endpoint unless explicitly required for specific sandbox functionality (which is usually not the case).

## 5. Operational Hygiene

- **Logging** Use [`LOG_HTTP_REQUESTS`](configuration.md#LOG_HTTP_REQUESTS), [`LOG_MCP_REQUESTS`](configuration.md#LOG_MCP_REQUESTS), [`LOG_TOOLCALLS`](configuration.md#LOG_TOOLCALLS) (see [Configuration](configuration.md)) for auditing, but be cautious not to log sensitive data.
- **Sandbox Image Hardening** Use minimal base images (e.g., Alpine, distroless) and keep images updated. Avoid installing unnecessary tools or packages.
