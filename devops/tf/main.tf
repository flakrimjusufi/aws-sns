terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "4.23.0"
    }
  }
  required_version = ">= 1.1.0"

  cloud {
    organization = "neocharge"

    workspaces {
      name = "joe-sandbox"
    }
  }
}

provider "aws" {
  region = "us-west-1"
}