package provider

import (
	"context"
	"fmt"

	"github.com/brockleyai/brockleyai/internal/model"
)

// BedrockProvider implements model.LLMProvider for AWS Bedrock.
// Phase 0: stub implementation that returns an error indicating full support
// is not yet available. The struct fields are defined for future implementation.
type BedrockProvider struct {
	Region    string
	AccessKey string
	SecretKey string
}

var _ model.LLMProvider = (*BedrockProvider)(nil)

// NewBedrockProvider creates a new Bedrock provider stub.
func NewBedrockProvider(region, accessKey, secretKey string) *BedrockProvider {
	return &BedrockProvider{
		Region:    region,
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
}

func (p *BedrockProvider) Name() string {
	return "bedrock"
}

func (p *BedrockProvider) Complete(_ context.Context, _ *model.CompletionRequest) (*model.CompletionResponse, error) {
	return nil, fmt.Errorf("bedrock provider requires AWS credentials configuration — not yet fully implemented")
}

func (p *BedrockProvider) Stream(_ context.Context, _ *model.CompletionRequest) (<-chan model.StreamChunk, error) {
	return nil, fmt.Errorf("bedrock provider requires AWS credentials configuration — not yet fully implemented")
}
