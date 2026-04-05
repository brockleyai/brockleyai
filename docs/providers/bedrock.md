# AWS Bedrock Provider

Config value: `"bedrock"`

**Status: Stub implementation.** The Bedrock provider is registered in the default provider registry but returns an error when called. The struct fields and configuration points are defined for future implementation.

## Current Behavior

Calling an LLM node with `"provider": "bedrock"` returns:

```
bedrock provider requires AWS credentials configuration -- not yet fully implemented
```

## Planned Configuration

| Field | Description |
|-------|-------------|
| `bedrock_region` | AWS region (e.g., `us-east-1`) |
| `api_key_ref` | Secret store reference for `<access_key>:<secret_key>` |

### Authentication (Planned)

AWS Bedrock uses SigV4 request signing. The provider will need AWS access key and secret key credentials, resolved via the secret store at runtime.

### Model IDs (Planned)

Bedrock uses provider-prefixed model IDs:

| Model | Bedrock Model ID |
|-------|-----------------|
| Claude Sonnet | `anthropic.claude-sonnet-4-20250514-v1:0` |
| Claude Haiku | `anthropic.claude-haiku-3-20250414-v1:0` |
| Titan Text | `amazon.titan-text-express-v1` |
| Llama 3 | `meta.llama3-70b-instruct-v1:0` |

### Environment Variables (Planned)

```bash
export BROCKLEY_SECRET_AWS_BEDROCK="<access_key>:<secret_key>"
```

## Example Node Config (Future)

```json
{
  "config": {
    "provider": "bedrock",
    "model": "anthropic.claude-sonnet-4-20250514-v1:0",
    "api_key_ref": "aws-bedrock",
    "bedrock_region": "us-east-1",
    "system_prompt": "You are a helpful assistant.",
    "user_prompt": "{{input.question}}",
    "variables": [
      {"name": "question", "schema": {"type": "string"}}
    ]
  }
}
```

## Workaround

Until Bedrock is fully implemented, you can access Bedrock models through OpenRouter or by running a local OpenAI-compatible proxy (such as [LiteLLM](https://github.com/BerriAI/litellm)) and using the `openai` provider with a custom `base_url`.

## See Also

- [LLM Node Reference](../nodes/llm.md) -- full LLM node configuration and output modes
- [Providers Overview](overview.md) -- how providers work, secret resolution
- [Provider Interface](provider-interface.md) -- Complete/Stream methods, error codes
- [Supported Providers](supported.md) -- comparison of all providers
- [Custom Providers](custom.md) -- implementing your own provider
