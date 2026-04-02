variable "aws_region" {
  description = "AWS region to provision replay instances in."
  default     = "us-east-1"

  validation {
    condition     = length(trimspace(var.aws_region)) > 0 && can(regex("^[a-z]{2}(-gov)?-[a-z]+-[0-9]+$", trimspace(var.aws_region)))
    error_message = "aws_region must be a non-empty AWS region identifier."
  }
}

variable "aws_release_host_lock_path" {
  description = "Absolute or module-relative path to the checked-in AWS AMI lock document."
  type        = string

  validation {
    condition     = length(trimspace(var.aws_release_host_lock_path)) > 0
    error_message = "aws_release_host_lock_path must be non-empty."
  }
}

variable "infra_repo_url" {
  description = "URL of the source repository containing this IaC."
  default     = "https://github.com/lattice-substrate/json-canon"
}

variable "infra_repo_commit" {
  description = "git SHA of this repository at provisioning time (set by release script)."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-f]{40}$", trimspace(var.infra_repo_commit)))
    error_message = "infra_repo_commit must be a 40-character lowercase git SHA."
  }
}

variable "provider_lock_sha256" {
  description = "SHA-256 of infra/.terraform.lock.hcl; verified by the release gate."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-f]{64}$", trimspace(var.provider_lock_sha256)))
    error_message = "provider_lock_sha256 must be a 64-character lowercase SHA-256 digest."
  }
}

variable "tofu_version" {
  description = "OpenTofu CLI version used to apply this configuration."
  default     = "1.8.0"
}
