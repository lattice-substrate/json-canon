output "x86_64_public_ip" {
  description = "Public IP of the x86_64 replay instance."
  value       = aws_instance.x86_replay.public_ip
}

output "arm64_public_ip" {
  description = "Public IP of the arm64 (Graviton2) replay instance."
  value       = aws_instance.arm64_replay.public_ip
}

output "x86_64_ami" {
  description = "AMI ID used for the x86_64 instance."
  value       = aws_instance.x86_replay.ami
}

output "arm64_ami" {
  description = "AMI ID used for the arm64 instance."
  value       = aws_instance.arm64_replay.ami
}

output "x86_64_instance_type" {
  description = "EC2 instance type of the x86_64 instance."
  value       = aws_instance.x86_replay.instance_type
}

output "arm64_instance_type" {
  description = "EC2 instance type of the arm64 instance."
  value       = aws_instance.arm64_replay.instance_type
}
