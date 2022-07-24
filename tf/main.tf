terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "4.23.0"
    }
  }
}

provider "aws" {

  default_tags {
    tags = {
      Terraform  = "true"
      Repository = "joe-sandbox"
    }
  }
}