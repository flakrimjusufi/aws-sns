package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
	vault "github.com/hashicorp/vault/api"
	vault_aws "github.com/hashicorp/vault/api/auth/aws"
)

func PublishMessage(svc snsiface.SNSAPI, msg, phoneNumber *string) (*sns.PublishOutput, error) {
	result, err := svc.Publish(&sns.PublishInput{
		Message:     msg,
		PhoneNumber: phoneNumber,
	})

	return result, err
}

func createVaultClient() (*vault.Client, error) {
	//configure client
	config := vault.DefaultConfig()
	config.Address = "https://vault.dev.neocharge.io" //TODO: replace with active.vault.service.consul

	//create client
	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	auth, err := vault_aws.NewAWSAuth(vault_aws.WithIAMAuth())
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
	log.Println("Retrieve()")
	rv := credentials.Value{
		ProviderName: "Vault",
	}

	args := make(map[string]interface{})
	args["ttl"] = vp.TTL

	log.Println("vp.VaultClient.Logical().Write(vp.CredentialPath, args)")
	resp, err := vp.VaultClient.Logical().Write(vp.CredentialPath, args)
	if err != nil {
		log.Println("aaaaa")
		return rv, err
	}

	// set expiration time via credentials.Expiry with a 10 second window
	vp.SetExpiration(time.Now().Add(time.Duration(resp.LeaseDuration)*time.Second), time.Duration(10*time.Second))

	rv.AccessKeyID = resp.Data["access_key"].(string)
	rv.SecretAccessKey = resp.Data["secret_key"].(string)
	rv.SessionToken = resp.Data["security_token"].(string)

	log.Println("credentials: ", rv.AccessKeyID, rv.SecretAccessKey, rv.SessionToken)

	return rv, nil
}

func main() {

	randomNumber := strconv.Itoa(int(GenerateRandomNumber()))

	msgPtr := flag.String("m", randomNumber, "The message to send to the user")
	phoneNumber := flag.String("n", "+16366146678",
		"The phone number you want to send message to in E.164 format")

	flag.Parse()

	if *msgPtr == "" || *phoneNumber == "" {
		log.Fatalf("You must supply a message and a phone number")
	}

	client, err := createVaultClient()
	if err != nil {
		log.Fatalf("Unable to create vault client: %s", err)
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Region:      aws.String("us-west-2"), //SMS must come from us-west-2 region!
			Credentials: credentials.NewCredentials(NewVaultProvider(client, "aws", "sms_sender")),
		},
	}))

	svc := sns.New(sess)

	result, err := PublishMessage(svc, msgPtr, phoneNumber)
	if err != nil {
		fmt.Println("Got an error publishing the message:")
		fmt.Println(err)
		return
	}
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Println(*result.MessageId)
}

func GenerateRandomNumber() uint16 {
	var n uint16
	binary.Read(rand.Reader, binary.LittleEndian, &n)
	return n
}
