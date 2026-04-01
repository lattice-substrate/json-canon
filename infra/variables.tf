variable "aws_region" {
  description = "AWS region to provision replay instances in."
  default     = "us-east-1"
}

variable "ssh_public_key" {
  description = "PEM public key content for the EC2 key pair (contents of ~/.ssh/id_rsa.pub)."
  type        = string
}

variable "infra_repo_url" {
  description = "URL of the source repository containing this IaC."
  default     = "https://github.com/lattice-substrate/json-canon"
}

variable "infra_repo_commit" {
  description = "git SHA of this repository at provisioning time (set by release script)."
  type        = string
}

variable "provider_lock_sha256" {
  description = "SHA-256 of infra/.terraform.lock.hcl; verified by the release gate."
  type        = string
}

variable "tofu_version" {
  description = "OpenTofu CLI version used to apply this configuration."
  default     = "1.8.0"
}

variable "ssh_ingress_cidr" {
  description = "CIDR block allowed to SSH to the replay instances."
  type        = string
}
