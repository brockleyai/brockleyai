package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

func stringValue(s string) types.String {
	return types.StringValue(s)
}
