terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  required_version = ">= 1.6.0"
}

provider "aws" {
  region = var.aws_region
}

resource "aws_key_pair" "replay" {
  key_name   = "jcs-replay-key"
  public_key = var.ssh_public_key
}

resource "aws_security_group" "replay" {
  name        = "jcs-replay-sg"
  description = "Allow SSH in, all egress for jcs offline replay nodes."

  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.ssh_ingress_cidr]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
