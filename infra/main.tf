terraform {
  backend "s3" {}

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.100"
    }
  }
  required_version = ">= 1.6.0"
}

provider "aws" {
  region = var.aws_region
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_prefix_list" "s3" {
  name = "com.amazonaws.${var.aws_region}.s3"
}

data "aws_iam_policy_document" "replay_instance_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

locals {
  replay_subnet_azs = slice(data.aws_availability_zones.available.names, 0, min(length(data.aws_availability_zones.available.names), 2))
  replay_subnets = {
    for index, az in local.replay_subnet_azs : format("az%02d", index) => {
      availability_zone = az
      cidr_block        = cidrsubnet("10.42.0.0/16", 4, index)
    }
  }
}

resource "aws_vpc" "replay" {
  cidr_block           = "10.42.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name    = "jcs-replay-vpc"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_subnet" "replay" {
  for_each = local.replay_subnets

  vpc_id                  = aws_vpc.replay.id
  availability_zone       = each.value.availability_zone
  cidr_block              = each.value.cidr_block
  map_public_ip_on_launch = false

  tags = {
    Name    = "jcs-replay-${each.key}"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_route_table" "replay" {
  vpc_id = aws_vpc.replay.id

  tags = {
    Name    = "jcs-replay-private"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_route_table_association" "replay" {
  for_each = aws_subnet.replay

  subnet_id      = each.value.id
  route_table_id = aws_route_table.replay.id
}

resource "aws_security_group" "replay_instance" {
  name        = "jcs-replay-instance-sg"
  description = "Restrict official replay hosts to SSM and S3 endpoint egress."
  vpc_id      = aws_vpc.replay.id

  egress {
    description     = "HTTPS to S3 gateway endpoint"
    from_port       = 443
    to_port         = 443
    protocol        = "tcp"
    prefix_list_ids = [data.aws_prefix_list.s3.id]
  }

  tags = {
    Name    = "jcs-replay-instance-sg"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_security_group" "vpc_endpoint" {
  name        = "jcs-replay-endpoint-sg"
  description = "Allow replay hosts to reach AWS private interface endpoints."
  vpc_id      = aws_vpc.replay.id

  egress {
    description = "Endpoint response traffic inside replay VPC"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = [aws_vpc.replay.cidr_block]
  }

  tags = {
    Name    = "jcs-replay-endpoint-sg"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_security_group_rule" "replay_instance_to_vpc_endpoint_https" {
  type                     = "egress"
  description              = "HTTPS to interface endpoints"
  from_port                = 443
  to_port                  = 443
  protocol                 = "tcp"
  security_group_id        = aws_security_group.replay_instance.id
  source_security_group_id = aws_security_group.vpc_endpoint.id
}

resource "aws_security_group_rule" "vpc_endpoint_from_replay_instance_https" {
  type                     = "ingress"
  description              = "Replay hosts to AWS endpoints"
  from_port                = 443
  to_port                  = 443
  protocol                 = "tcp"
  security_group_id        = aws_security_group.vpc_endpoint.id
  source_security_group_id = aws_security_group.replay_instance.id
}

resource "aws_vpc_endpoint" "ssm" {
  vpc_id              = aws_vpc.replay.id
  service_name        = "com.amazonaws.${var.aws_region}.ssm"
  vpc_endpoint_type   = "Interface"
  private_dns_enabled = true
  subnet_ids          = [for subnet in aws_subnet.replay : subnet.id]
  security_group_ids  = [aws_security_group.vpc_endpoint.id]

  tags = {
    Name    = "jcs-replay-ssm"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_vpc_endpoint" "ssmmessages" {
  vpc_id              = aws_vpc.replay.id
  service_name        = "com.amazonaws.${var.aws_region}.ssmmessages"
  vpc_endpoint_type   = "Interface"
  private_dns_enabled = true
  subnet_ids          = [for subnet in aws_subnet.replay : subnet.id]
  security_group_ids  = [aws_security_group.vpc_endpoint.id]

  tags = {
    Name    = "jcs-replay-ssmmessages"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_vpc_endpoint" "ec2messages" {
  vpc_id              = aws_vpc.replay.id
  service_name        = "com.amazonaws.${var.aws_region}.ec2messages"
  vpc_endpoint_type   = "Interface"
  private_dns_enabled = true
  subnet_ids          = [for subnet in aws_subnet.replay : subnet.id]
  security_group_ids  = [aws_security_group.vpc_endpoint.id]

  tags = {
    Name    = "jcs-replay-ec2messages"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.replay.id
  service_name      = "com.amazonaws.${var.aws_region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.replay.id]

  tags = {
    Name    = "jcs-replay-s3"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_iam_role" "replay_instance" {
  name               = "jcs-replay-instance-role"
  assume_role_policy = data.aws_iam_policy_document.replay_instance_assume_role.json

  tags = {
    Name    = "jcs-replay-instance-role"
    Purpose = "jcs-official-aws-release"
  }
}

resource "aws_iam_role_policy_attachment" "replay_instance_ssm" {
  role       = aws_iam_role.replay_instance.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "replay_instance" {
  name = "jcs-replay-instance-profile"
  role = aws_iam_role.replay_instance.name
}
