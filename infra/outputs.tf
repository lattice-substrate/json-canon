output "provisioned_hosts" {
  description = "Provisioned official AWS release hosts keyed by host_id."
  sensitive   = true
  value = {
    for host_id, inst in aws_instance.release_host : host_id => {
      host_id           = host_id
      node_id           = local.aws_release_hosts[host_id].node_id
      architecture      = local.aws_release_hosts[host_id].architecture
      private_ip        = inst.private_ip
      image_id          = inst.ami
      instance_id       = inst.id
      availability_zone = inst.availability_zone
      instance_type     = inst.instance_type
      distro            = local.aws_release_hosts[host_id].distro
      kernel_family     = local.aws_release_hosts[host_id].kernel_family
    }
  }
}
