locals {
  aws_release_hosts_raw = jsondecode(file("${path.module}/aws_release_hosts.json"))
  aws_release_hosts = {
    for host in local.aws_release_hosts_raw.hosts : host.host_id => host
  }
}

data "aws_ami" "release_host" {
  for_each    = local.aws_release_hosts
  most_recent = true
  owners      = [each.value.ami_owner]

  filter {
    name   = "name"
    values = [each.value.ami_name]
  }

  filter {
    name   = "architecture"
    values = [each.value.architecture]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_instance" "release_host" {
  for_each = local.aws_release_hosts

  ami                         = data.aws_ami.release_host[each.key].id
  instance_type               = each.value.instance_type
  associate_public_ip_address = true
  key_name                    = aws_key_pair.replay.key_name
  vpc_security_group_ids      = [aws_security_group.replay.id]

  root_block_device {
    volume_size = 10
    volume_type = "gp3"
  }

  tags = {
    Name         = each.key
    Purpose      = "jcs-official-aws-release"
    Architecture = each.value.architecture
    NodeID       = each.value.node_id
    Distro       = each.value.distro
    KernelFamily = each.value.kernel_family
  }
}
