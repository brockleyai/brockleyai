# Secrets and API Key Configuration

This guide covers how to configure LLM API keys, custom secrets, and security best practices for Brockley deployments.

## How Secrets Work in Brockley

Brockley uses two kinds of secrets:

1. **Platform secrets** -- API keys for authenticating with Brockley itself (`BROCKLEY_API_KEYS`) and the encryption key for secrets at rest (`BROCKLEY_ENCRYPTION_KEY`).
2. **LLM provider secrets** -- API keys for OpenAI, Anthropic, Google, OpenRouter, Bedrock, and other providers. These follow the `BROCKLEY_SECRET_*` convention.

## The BROCKLEY_SECRET_* Convention

Any environment variable prefixed with `BROCKLEY_SECRET_` is treated as a secret that can be referenced in graph definitions. The server and worker processes read these at startup.

| Environment Variable | What It Provides |
|---------------------|-----------------|
| `BROCKLEY_SECRET_OPENAI_API_KEY` | OpenAI API key |
| `BROCKLEY_SECRET_ANTHROPIC_API_KEY` | Anthropic API key |
| `BROCKLEY_SECRET_GOOGLE_API_KEY` | Google AI (Gemini) API key |
| `BROCKLEY_SECRET_OPENROUTER_API_KEY` | OpenRouter API key |
| `BROCKLEY_SECRET_CUSTOM_HEADER_TOKEN` | Any custom secret you define |

The naming after the `BROCKLEY_SECRET_` prefix is up to you. Use descriptive names that match how you reference them in graphs.

## Referencing Secrets in Graphs

In graph definitions, LLM nodes reference secrets by name using the `api_key_ref` field instead of embedding raw keys:

```json
{
  "type": "llm-call",
  "config": {
    "provider": "openai",
    "model": "gpt-4o",
    "api_key_ref": "OPENAI_API_KEY"
  }
}
```

The `api_key_ref` value maps to the environment variable `BROCKLEY_SECRET_OPENAI_API_KEY`. Brockley strips the `BROCKLEY_SECRET_` prefix when matching.

This means:
- `api_key_ref: "OPENAI_API_KEY"` reads from `BROCKLEY_SECRET_OPENAI_API_KEY`
- `api_key_ref: "ANTHROPIC_API_KEY"` reads from `BROCKLEY_SECRET_ANTHROPIC_API_KEY`
- `api_key_ref: "MY_CUSTOM_KEY"` reads from `BROCKLEY_SECRET_MY_CUSTOM_KEY`

---

## Kubernetes Secrets Approach

The recommended way to manage LLM keys in Kubernetes is with a dedicated Secret resource.

### Step 1: Create the Secret

```bash
kubectl create secret generic brockley-llm-keys \
  --namespace brockley \
  --from-literal=BROCKLEY_SECRET_OPENAI_API_KEY="sk-your-openai-key" \
  --from-literal=BROCKLEY_SECRET_ANTHROPIC_API_KEY="sk-ant-your-anthropic-key" \
  --from-literal=BROCKLEY_SECRET_GOOGLE_API_KEY="your-google-key"
```

### Step 2: Inject into Deployments

Patch the server and worker deployments to mount the secret as environment variables:

```bash
kubectl set env deployment/brockley-server -n brockley --from=secret/brockley-llm-keys
kubectl set env deployment/brockley-worker -n brockley --from=secret/brockley-llm-keys
```

Both pods will restart automatically with the new environment variables.

### Step 3: Verify

```bash
kubectl rollout status deployment/brockley-server -n brockley
kubectl rollout status deployment/brockley-worker -n brockley
```

### Updating Secrets

To add or change a key:

```bash
kubectl create secret generic brockley-llm-keys \
  --namespace brockley \
  --from-literal=BROCKLEY_SECRET_OPENAI_API_KEY="sk-new-key" \
  --from-literal=BROCKLEY_SECRET_ANTHROPIC_API_KEY="sk-ant-new-key" \
  --from-literal=BROCKLEY_SECRET_GOOGLE_API_KEY="new-google-key" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Then restart the pods to pick up the change:

```bash
kubectl rollout restart deployment/brockley-server -n brockley
kubectl rollout restart deployment/brockley-worker -n brockley
```

---

## Using the Helm Chart's Secret Template

The Helm chart creates a Secret named `brockley-secret` for platform secrets (`BROCKLEY_API_KEYS` and `BROCKLEY_ENCRYPTION_KEY`). You can extend this for LLM keys by adding them to the server and worker environment variables via the `env` field in values:

```yaml
server:
  env:
    BROCKLEY_SECRET_OPENAI_API_KEY: "sk-your-key"

worker:
  env:
    BROCKLEY_SECRET_OPENAI_API_KEY: "sk-your-key"
```

**Warning:** This puts secrets in the ConfigMap (plaintext in etcd). For production, use the Kubernetes Secrets approach above or an external secret manager.

---

## YAML Manifest Approach

For declarative GitOps workflows, define the secret in a YAML manifest. Do not commit this file to version control.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: brockley-llm-keys
  namespace: brockley
type: Opaque
stringData:
  BROCKLEY_SECRET_OPENAI_API_KEY: "sk-your-openai-key"
  BROCKLEY_SECRET_ANTHROPIC_API_KEY: "sk-ant-your-anthropic-key"
  BROCKLEY_SECRET_GOOGLE_API_KEY: "your-google-key"
  BROCKLEY_SECRET_OPENROUTER_API_KEY: "sk-or-your-key"
```

Apply it:

```bash
kubectl apply -f brockley-llm-secrets.yaml
```

Then patch deployments:

```bash
kubectl set env deployment/brockley-server -n brockley --from=secret/brockley-llm-keys
kubectl set env deployment/brockley-worker -n brockley --from=secret/brockley-llm-keys
```

---

## External Secret Managers

For production environments, consider using an external secret manager with the [External Secrets Operator](https://external-secrets.io/) to sync secrets from:

- **AWS Secrets Manager** or **SSM Parameter Store**
- **GCP Secret Manager**
- **Azure Key Vault**
- **HashiCorp Vault**

Example ExternalSecret for AWS Secrets Manager:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: brockley-llm-keys
  namespace: brockley
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: ClusterSecretStore
  target:
    name: brockley-llm-keys
    creationPolicy: Owner
  data:
    - secretKey: BROCKLEY_SECRET_OPENAI_API_KEY
      remoteRef:
        key: brockley/openai-api-key
    - secretKey: BROCKLEY_SECRET_ANTHROPIC_API_KEY
      remoteRef:
        key: brockley/anthropic-api-key
```

---

## Platform Secrets

### API Keys (BROCKLEY_API_KEYS)

Controls access to the Brockley API. When set, all requests must include `Authorization: Bearer <key>`.

Generate a key:

```bash
openssl rand -hex 24
```

Set via Helm:

```bash
helm install brockley deploy/helm/brockley \
  --set auth.apiKeys="your-generated-key"
```

Multiple keys (for rotation):

```bash
--set auth.apiKeys="new-key,old-key"
```

### Encryption Key (BROCKLEY_ENCRYPTION_KEY)

Encrypts secrets at rest in PostgreSQL. Must be a 32-byte key, base64-encoded.

Generate:

```bash
openssl rand -base64 32
```

Set via Helm:

```bash
helm install brockley deploy/helm/brockley \
  --set encryptionKey="your-base64-key"
```

**Do not lose this key.** If you rotate it, existing encrypted secrets become unreadable. Plan a migration before rotating.

---

## Security Best Practices

1. **Never put raw API keys in graph definitions.** Always use `api_key_ref` to reference secrets by name.

2. **Never commit secrets to git.** Use `--set` overrides, sealed secrets, or an external secret manager.

3. **Use Kubernetes RBAC** to restrict who can read secrets in the `brockley` namespace:

   ```bash
   kubectl auth can-i get secrets -n brockley --as=system:serviceaccount:default:default
   ```

4. **Enable etcd encryption at rest** in your Kubernetes cluster. This encrypts all Secrets stored in etcd. See your cloud provider's docs:
   - [GKE: Application-layer Secrets encryption](https://cloud.google.com/kubernetes-engine/docs/how-to/encrypting-secrets)
   - [EKS: Envelope encryption](https://docs.aws.amazon.com/eks/latest/userguide/enable-kms.html)
   - [AKS: etcd encryption](https://learn.microsoft.com/en-us/azure/aks/use-kms-etcd-encryption)

5. **Rotate keys regularly.** Use multiple API keys (`BROCKLEY_API_KEYS=new-key,old-key`) during rotation windows.

6. **Set `BROCKLEY_ENCRYPTION_KEY`** to encrypt secrets at rest in PostgreSQL. Without it, secrets stored via the API are kept in plaintext.

7. **Secrets never appear in logs.** Brockley logs `api_key_ref` names, never values. Verify this by checking server logs:

   ```bash
   kubectl logs deployment/brockley-server -n brockley | grep -i "secret\|key"
   ```

   You should see references like `api_key_ref=OPENAI_API_KEY`, never the actual key value.

---

## Verifying Secrets Are Working

After configuring secrets, test that the server can resolve them:

```bash
# Port-forward to the server
kubectl port-forward svc/brockley-server -n brockley 8000:8000 &

# Create a simple graph with an LLM node that uses the secret
curl -X POST http://localhost:8000/api/v1/graphs \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "secret-test",
    "nodes": [
      {
        "id": "llm1",
        "type": "llm-call",
        "config": {
          "provider": "openai",
          "model": "gpt-4o-mini",
          "api_key_ref": "OPENAI_API_KEY",
          "prompt": "Say hello in one word."
        }
      }
    ],
    "edges": []
  }'

# Execute the graph
curl -X POST http://localhost:8000/api/v1/graphs/secret-test/execute \
  -H "Authorization: Bearer YOUR_API_KEY"
```

If the secret is missing or misconfigured, the execution will fail with an error indicating the secret reference could not be resolved.

## Next Steps

- [Helm values reference](helm.md) -- full chart configuration
- [Configuration reference](configuration.md) -- all environment variables
- [Cloud deployment overview](cloud-deploy.md) -- infrastructure provisioning
- [Monitoring](monitoring.md) -- observability setup
