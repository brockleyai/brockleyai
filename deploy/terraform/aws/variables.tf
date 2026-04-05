variable "region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-east-1"
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

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "database_instance_class" {
  description = "RDS instance class. Overrides the default for the selected size."
  type        = string
  default     = ""
}

variable "database_allocated_storage" {
  description = "RDS allocated storage in GB"
  type        = number
  default     = 20
}

variable "redis_node_type" {
  description = "ElastiCache node type. Overrides the default for the selected size."
  type        = string
  default     = ""
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
  description = "Tags applied to all AWS resources"
  type        = map(string)
  default     = {}
}
