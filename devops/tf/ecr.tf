variable "aws_ecr_repository_name" {
  type = string
}

resource "aws_ecr_repository" "default" {
  name                 = var.aws_ecr_repository_name
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  lifecycle {
    prevent_destroy = false
  }
}

output "ecr_respository_url" {
  value = aws_ecr_repository.default.repository_url
}