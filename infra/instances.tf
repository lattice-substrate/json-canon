locals {
  aws_release_hosts_lock_raw = jsondecode(file(var.aws_release_host_lock_path))
  aws_release_hosts = {
    for host in local.aws_release_hosts_lock_raw.hosts : host.host_id => host
  }
  replay_subnet_ids = [for key in sort(keys(aws_subnet.replay)) : aws_subnet.replay[key].id]
  aws_release_host_ids = sort(keys(local.aws_release_hosts))
  aws_release_host_subnet_index = {
    for index, host_id in local.aws_release_host_ids : host_id => index % length(local.replay_subnet_ids)
  }
}

resource "aws_instance" "release_host" {
  for_each = local.aws_release_hosts

  ami                         = each.value.ami_id
  instance_type               = each.value.instance_type
  subnet_id                   = local.replay_subnet_ids[local.aws_release_host_subnet_index[each.key]]
  associate_public_ip_address = false
  iam_instance_profile        = aws_iam_instance_profile.replay_instance.name
  vpc_security_group_ids      = [aws_security_group.replay_instance.id]

  root_block_device {
    volume_size = 10
    volume_type = "gp3"
  }

  metadata_options {
    http_tokens = "required"
  }

  user_data = <<-EOT
    #!/bin/sh
    set -eu
    if command -v systemctl >/dev/null 2>&1; then
      systemctl enable amazon-ssm-agent >/dev/null 2>&1 || true
      systemctl restart amazon-ssm-agent >/dev/null 2>&1 || true
    else
      service amazon-ssm-agent start >/dev/null 2>&1 || true
    fi
  EOT

  depends_on = [
    aws_vpc_endpoint.s3,
    aws_vpc_endpoint.ssm,
    aws_vpc_endpoint.ssmmessages,
    aws_vpc_endpoint.ec2messages,
    aws_iam_role_policy_attachment.replay_instance_ssm,
  ]

  tags = {
    Name         = each.key
    Purpose      = "jcs-official-aws-release"
    Architecture = each.value.architecture
    NodeID       = each.value.node_id
    Distro       = each.value.distro
    KernelFamily = each.value.kernel_family
    Transport    = "ssm"
    Subnet       = "private"
  }
}
