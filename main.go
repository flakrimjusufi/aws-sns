package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
	"github.com/aws/aws-sdk-go/service/sts"
	vault "github.com/hashicorp/vault/api"
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

	//authenticate client
	err = AWSIamLogin(client, "aws-ec2", "SERVERID", "nomad-job-role")
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate vault client with IAM: %w", err)
	}

	return client, nil
}

// AWSIamLogin will create a Vault client, login via an AWS role, and return a valid Vault token and client that can be
// used to get secrets.
// The authProvider is likely "aws". It's the "Path" column as described in these docs:
// https://www.vaultproject.io/api/auth/aws#login.
// The serverID is an optional value to be placed in the X-Vault-AWS-IAM-Server-ID header of the HTTP request.
// The role is an AWS IAM role. It needs to be able to read secrets from Vault.
func AWSIamLogin(client *vault.Client, authProvider, serverID, role string) (err error) {

	// Acquire an AWS session.
	sess, err := session.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create a Go structure to talk to the AWS token service.
	tokenService := sts.New(sess)

	// Create a request to the token service that will ask for the current host's identity.
	request, _ := tokenService.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})

	// Add an server ID IAM header, if present.
	if serverID != "" {
		request.HTTPRequest.Header.Add("X-Vault-AWS-IAM-Server-ID", serverID)
	}

	// Sign the request to the AWS token service.
	if err = request.Sign(); err != nil {
		return fmt.Errorf("failed to sign AWS identity request: %w", err)
	}

	// JSON marshal the headers.
	var headers []byte
	if headers, err = json.Marshal(request.HTTPRequest.Header); err != nil {
		return fmt.Errorf("failed to JSON marshal HTTP headers for AWS identity request: %w", err)
	}

	// Read the body of the request.
	var body []byte
	if body, err = ioutil.ReadAll(request.HTTPRequest.Body); err != nil {
		return fmt.Errorf("failed to JSON marshal HTTP body for AWS identity request: %w", err)
	}

	// Create the data to write to Vault.
	data := make(map[string]interface{})
	data["iam_http_request_method"] = request.HTTPRequest.Method
	data["iam_request_url"] = base64.StdEncoding.EncodeToString([]byte(request.HTTPRequest.URL.String()))
	data["iam_request_headers"] = base64.StdEncoding.EncodeToString(headers)
	data["iam_request_body"] = base64.StdEncoding.EncodeToString(body)
	data["role"] = role

	// Create the path to write to for Vault.
	// The authProvider is the value referenced in the "Path" column in this documentation. It's likely "aws".
	// https://www.vaultproject.io/api/auth/aws#login
	path := fmt.Sprintf("auth/%s/login", authProvider)

	// Write the AWS token service request to Vault.
	secret, err := client.Logical().Write(path, data)
	if err != nil {
		return fmt.Errorf("failed to write data to Vault to get token: %w", err)
	}

	if secret == nil {
		return fmt.Errorf("failed to get token from Vault: %w", err)
	}

	// Get the Vault token from the response.
	token, err := secret.TokenID()
	if err != nil {
		return fmt.Errorf("failed to get token from Vault response: %w", err)
	}

	// Set the token for the client as the one it just received.
	client.SetToken(token)
	log.Println("Vault token! ", token)

	return nil
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

	args := make(map[string]interface{})
	args["ttl"] = vp.TTL

	resp, err := vp.VaultClient.Logical().Write(vp.CredentialPath, args)
	if err != nil {
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
