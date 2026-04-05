variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region for all resources"
  type        = string
  default     = "us-central1"
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

variable "database_tier" {
  description = "Cloud SQL machine type. Overrides the default for the selected size."
  type        = string
  default     = ""
}

variable "database_disk_size_gb" {
  description = "Cloud SQL disk size in GB"
  type        = number
  default     = 20
}

variable "redis_memory_size_gb" {
  description = "Memorystore Redis instance size in GB"
  type        = number
  default     = 1
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

variable "labels" {
  description = "Labels applied to all GCP resources"
  type        = map(string)
  default     = {}
}
