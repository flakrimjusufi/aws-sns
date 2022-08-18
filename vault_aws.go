package main

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	vault "github.com/hashicorp/vault/api"
)

func NewAwsSession() (*session.Session, error) {
	client, err := VaultClient()
	if err != nil {
		return nil, err
	}

	return session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Region:      aws.String("us-west-2"), //SMS must come from us-west-2 region!
			Credentials: credentials.NewCredentials(newvaultProvider(client, "aws", "sms_sender")),
		},
	})
}

// The vaultProvider object implements the AWS SDK `credentials.Provider`
// interface. Use the `NewvaultProvider` function to construct the object with
// default settings, or if you need to configure the `vault.Client` object,
// TTL, or path yourself, you can build the object by hand.
type vaultProvider struct {
	// The full Vault API path to the STS credentials endpoint.
	CredentialPath string

	// The `vault.Client` object used to interact with Vault.
	VaultClient *vault.Client

	// compose with credentials.Expiry to get free IsExpired()
	credentials.Expiry
}

// Creates a new vaultProvider. Supply the path where the AWS secrets engine
// is mounted as well as the role name to fetch from. The vaultProvider is
// initialized with a default client, which uses the VAULT_ADDR and VAULT_TOKEN
// environment variables to configure itself. This also sets a default TTL of
// 30 minutes for the credentials' lifetime.
func newvaultProvider(client *vault.Client, enginePath string, roleName string) *vaultProvider {
	return &vaultProvider{
		CredentialPath: (enginePath + "/creds/" + roleName),
		VaultClient:    client,
	}
}

// Implements the Retrieve() function for the AWS SDK credentials.Provider
// interface.
func (vp *vaultProvider) Retrieve() (credentials.Value, error) {
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
