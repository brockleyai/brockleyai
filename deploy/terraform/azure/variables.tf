variable "location" {
  description = "Azure region for all resources"
  type        = string
  default     = "eastus"
}

variable "name" {
  description = "Base name for all resources (e.g. 'brockley-prod')"
  type        = string
  default     = "brockley"
}

variable "size" {
  description = "Deployment size preset: starter, standard, or performance"
  type        = string
  default     = "standard"

  validation {
    condition     = contains(["starter", "standard", "performance"], var.size)
    error_message = "size must be one of: starter, standard, performance"
  }
}

variable "resource_group_name" {
  description = "Name of the Azure resource group. Created if it does not exist."
  type        = string
  default     = ""
}

variable "vnet_address_space" {
  description = "Address space for the VNet"
  type        = string
  default     = "10.0.0.0/16"
}

variable "database_sku" {
  description = "Azure Database for PostgreSQL Flexible Server SKU. Overrides the default for the selected size."
  type        = string
  default     = ""
}

variable "database_storage_mb" {
  description = "PostgreSQL storage size in MB"
  type        = number
  default     = 32768
}

variable "redis_sku" {
  description = "Azure Cache for Redis SKU (Basic, Standard, Premium). Overrides the default for the selected size."
  type        = string
  default     = ""
}

variable "redis_capacity" {
  description = "Azure Cache for Redis capacity (0-6 for Basic/Standard, 1-5 for Premium)"
  type        = number
  default     = 0
}

variable "helm_chart_path" {
  description = "Path to the Brockley Helm chart (local or repo URL)"
  type        = string
  default     = "../helm/brockley"
}

variable "brockley_image_tag" {
  description = "Docker image tag for Brockley components. Defaults to chart appVersion."
  type        = string
  default     = ""
}

variable "extra_helm_values" {
  description = "Additional Helm values to merge (YAML string)"
  type        = string
  default     = ""
}

variable "tags" {
  description = "Tags applied to all Azure resources"
  type        = map(string)
  default     = {}
}
