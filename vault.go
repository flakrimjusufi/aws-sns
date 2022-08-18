package main

import (
	"context"
	"fmt"

	vault "github.com/hashicorp/vault/api"
	vault_aws "github.com/hashicorp/vault/api/auth/aws"
)

func VaultClient() (*vault.Client, error) {
	//configure client
	config := vault.DefaultConfig()
	//config.Address = "https://active.vault.service.consul" //TODO: Consul DNS https cert chain
	config.Address = "https://vault.dev.neocharge.io"

	//create client
	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	//authenticate client via AWS IAM
	auth, err := vault_aws.NewAWSAuth(vault_aws.WithIAMAuth(), vault_aws.WithRole("nomad-job-role"), vault_aws.WithMountPath("aws-ec2"))
	if err != nil {
		return nil, fmt.Errorf("failed to setup vault client with IAM: %w", err)
	}

	//login with client
	_, err = client.Auth().Login(context.Background(), auth)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate vault client with IAM: %w", err)
	}

	return client, nil
}
