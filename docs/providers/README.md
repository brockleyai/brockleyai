# LLM Providers

Brockley supports multiple LLM providers through a unified interface. This section covers how providers work, what's available, and how to configure each one.

## Reading order

Start with the overview, check what's supported, then read the guide for your provider.

1. **[Provider Overview](overview.md)** -- How the provider system works: registry, API key resolution, and the request/response lifecycle.

2. **[Supported Providers](supported.md)** -- Comparison table of all providers: models, features, pricing tiers, and trade-offs.

3. **Pick your provider:**
   - **[OpenAI](openai.md)** -- GPT-4o, GPT-4, GPT-3.5, and other OpenAI models.
   - **[Anthropic](anthropic.md)** -- Claude Opus, Sonnet, and Haiku models.
   - **[Google](google.md)** -- Gemini models via the Google AI API.
   - **[OpenRouter](openrouter.md)** -- Access to 100+ models through a single API key, including free tiers.
   - **[AWS Bedrock](bedrock.md)** -- Run models in your own AWS account via Bedrock.

4. **Extending:**
   - **[Custom Providers](custom.md)** -- Build your own provider implementation for self-hosted or unsupported models.
   - **[Provider Interface](provider-interface.md)** -- The Go interface specification that all providers implement.

## Where to go next

- **[LLM Node](../nodes/llm.md)** -- Configure LLM nodes to use these providers.
- **[Superagent Node](../nodes/superagent.md)** -- Superagents also use providers for their LLM calls.
- **[Secrets Management](../deployment/secrets.md)** -- How to manage API keys in production.
