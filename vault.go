package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	vault "github.com/hashicorp/vault/api"
	vault_aws "github.com/hashicorp/vault/api/auth/aws"
)

func VaultClient() (*vault.Client, error) {
	//configure client
	config := vault.DefaultConfig()
	config.Address = "https://active.vault.service.consul"

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

	_, err = client.Auth().Login(context.Background(), auth)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate vault client with IAM: %w", err)
	}

	return client, nil
}

// The VaultProvider object implements the AWS SDK `credentials.Provider`
// interface. Use the `NewVaultProvider` function to construct the object with
// default settings, or if you need to configure the `vault.Client` object,
// TTL, or path yourself, you can build the object by hand.
type VaultProvider struct {
	// The full Vault API path to the STS credentials endpoint.
	CredentialPath string

	// The TTL of the STS credentials in the form of a Go duration string.
	TTL string

	// The `vault.Client` object used to interact with Vault.
	VaultClient *vault.Client

	// compose with credentials.Expiry to get free IsExpired()
	credentials.Expiry
}

// Creates a new VaultProvider. Supply the path where the AWS secrets engine
// is mounted as well as the role name to fetch from. The VaultProvider is
// initialized with a default client, which uses the VAULT_ADDR and VAULT_TOKEN
// environment variables to configure itself. This also sets a default TTL of
// 30 minutes for the credentials' lifetime.
func NewVaultProvider(client *vault.Client, enginePath string, roleName string) *VaultProvider {
	return &VaultProvider{
		CredentialPath: (enginePath + "/creds/" + roleName),
		TTL:            "30m",
		VaultClient:    client,
	}
}

// An extra shortcut to avoid needing to import credentials into your source
// file or call nested functions. Call this to return a new Credentials object
// using the VaultProvider.
func NewVaultProviderCredentials(client *vault.Client, enginePath string, roleName string) *credentials.Credentials {
	return credentials.NewCredentials(NewVaultProvider(client, enginePath, roleName))
}

// Implements the Retrieve() function for the AWS SDK credentials.Provider
// interface.
func (vp *VaultProvider) Retrieve() (credentials.Value, error) {
	rv := credentials.Value{
		ProviderName: "Vault",
	}

	resp, err := vp.VaultClient.Logical().Read(vp.CredentialPath)
	if err != nil {
		return rv, err
	}

	// set expiration time via credentials.Expiry with a 10 second window
	vp.SetExpiration(time.Now().Add(time.Duration(resp.LeaseDuration)*time.Second), time.Duration(10*time.Second))

	rv.AccessKeyID = resp.Data["access_key"].(string)
	rv.SecretAccessKey = resp.Data["secret_key"].(string)
	rv.SessionToken = resp.Data["security_token"].(string)

	return rv, nil
}
