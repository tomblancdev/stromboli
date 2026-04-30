# Stromboli on Kubernetes — reference manifests

These manifests are a **reference architecture**, not a turnkey deployment. They show how the moving parts (ConfigMap, Secret, Deployment, two Services, PVC) fit together so teams can adapt them to their cluster's patterns (Helm chart, Argo, Flux, External Secrets, etc.).

## Files

| File | Purpose |
|---|---|
| `namespace.yaml` | The `stromboli` namespace. |
| `configmap.yaml` | Non-secret config (matches `STROMBOLI_*` env vars elsewhere in the repo). |
| `secret.example.yaml` | **Template** for the JWT secret + Claude credentials. Replace before applying. |
| `deployment.yaml` | The Stromboli pod, ServiceAccount, and a PVC for session storage. |
| `service.yaml` | One ClusterIP Service for HTTP (`:8080`), one for metrics (`:9090`). |
| `kustomization.yaml` | Wires them together for `kubectl apply -k`. |

## The Podman socket problem

Stromboli spawns agents by talking to a **Podman socket** on the host. Kubernetes nodes don't run Podman by default, and Stromboli isn't a CRI client — it shells out to `podman` against `/run/podman/podman.sock`. That mismatch is the audit's "DaemonSet or privileged sidecar" caveat.

Three patterns work; pick whichever your cluster supports:

1. **Pinned-node hostPath** *(what these manifests assume)*: a subset of nodes runs Podman. Label them with `podman.io/socket=true`, install Podman + enable the user socket, and let the Deployment hostPath-mount `/run/podman/podman.sock`. Good for one-cluster-one-deployment setups; fragile if the labelled node goes away.

2. **Privileged sidecar**: a sidecar container in the pod runs `podman system service` and exposes the socket on a shared `emptyDir`. Stromboli reaches it via `unix:///shared/podman.sock`. Heavier (privileged sidecar, double the resource footprint) but has no node coupling.

3. **Off-K8s data plane**: run Stromboli on a VM or bare-metal host and only point K8s services at it. Often the cleanest path when you already manage Podman elsewhere.

The provided manifests use pattern (1) for clarity. If you adopt (2) or (3), drop `nodeSelector.podman.io/socket` from `deployment.yaml` and either swap the `hostPath` for an `emptyDir` (pattern 2) or drop the volume entirely and point `CONTAINER_HOST` at the off-cluster socket (pattern 3).

## Applying

```bash
# 1. Create your real secret. DO NOT commit the populated file.
cp deployments/kubernetes/secret.example.yaml /tmp/stromboli-secret.yaml
$EDITOR /tmp/stromboli-secret.yaml          # fill in JWT secret + Claude creds

# 2. Apply the namespace + manifests.
kubectl apply -f deployments/kubernetes/namespace.yaml
kubectl apply -f /tmp/stromboli-secret.yaml
kubectl apply -k deployments/kubernetes/    # configmap + deployment + services

# 3. Verify.
kubectl -n stromboli get pods,svc
kubectl -n stromboli logs deploy/stromboli
```

For production, replace step 1 with whatever secret manager your platform team uses (External Secrets Operator + Vault/AWS-SM, sealed-secrets, ArgoCD vault plugin, etc.) and remove `secret.example.yaml` from `kustomization.yaml`.

## Required cluster pieces

- A **default StorageClass** (the PVC requests 10Gi, RWO).
- An **Ingress** of your choice in front of the `stromboli` Service if external traffic is needed.
- A **Prometheus** that respects the `prometheus.io/scrape` annotations on the `stromboli-metrics` Service (most kube-prometheus-stack installs do).

## What's intentionally not here

- No HorizontalPodAutoscaler — Stromboli holds in-memory job state; horizontal scaling needs the future external job store.
- No NetworkPolicy — too cluster-specific. Add one that allows ingress to `:8080` from your ingress namespace and `:9090` from your monitoring namespace, plus egress to `kube-dns` and the OTLP collector.
- No PodDisruptionBudget — with `replicas: 1` and `strategy: Recreate`, a PDB doesn't help. If you move to (2) above and bump replicas, add one.
