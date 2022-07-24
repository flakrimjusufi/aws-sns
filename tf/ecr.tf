resource "aws_ecr_repository" "joe-sandbox" {
  name                 = "application-jsandbox"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  lifecycle {
    prevent_destroy = true
  }
}