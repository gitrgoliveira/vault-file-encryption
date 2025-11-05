terraform {
  required_version = ">= 1.5.0"

  required_providers {
    vault = {
      source  = "hashicorp/vault"
      version = "~> 3.23.0"
    }
  }
}

provider "vault" {
  address   = var.vault_address
  namespace = var.vault_namespace
  
  # Using certificate authentication for Terraform itself
  # In production, use appropriate auth method (token, OIDC, etc.)
}
