resource "aws_instance" "x86_replay" {
  ami                    = data.aws_ami.debian12_x86.id
  instance_type          = "t3a.small"
  associate_public_ip_address = true
  key_name               = aws_key_pair.replay.key_name
  vpc_security_group_ids = [aws_security_group.replay.id]

  root_block_device {
    volume_size = 10
    volume_type = "gp3"
  }

  tags = {
    Name    = "jcs-replay-x86_64"
    Purpose = "jcs-offline-replay"
  }
}

resource "aws_instance" "arm64_replay" {
  ami                    = data.aws_ami.debian12_arm64.id
  instance_type          = "t4g.small"
  associate_public_ip_address = true
  key_name               = aws_key_pair.replay.key_name
  vpc_security_group_ids = [aws_security_group.replay.id]

  root_block_device {
    volume_size = 10
    volume_type = "gp3"
  }

  tags = {
    Name    = "jcs-replay-arm64"
    Purpose = "jcs-offline-replay"
  }
}
