package main

import (
	"context"
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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/hashicorp/vault/api"
)

func PublishMessage(svc snsiface.SNSAPI, msg, phoneNumber *string) (*sns.PublishOutput, error) {
	result, err := svc.Publish(&sns.PublishInput{
		Message:     msg,
		PhoneNumber: phoneNumber,
	})

	return result, err
}

// AWSLogin will create a Vault client, login via an AWS role, and return a valid Vault token and client that can be
// used to get secrets.
// The authProvider is likely "aws". It's the "Path" column as described in these docs:
// https://www.vaultproject.io/api/auth/aws#login.
// The serverID is an optional value to be placed in the X-Vault-AWS-IAM-Server-ID header of the HTTP request.
// The role is an AWS IAM role. It needs to be able to read secrets from Vault.
func AWSLogin(authProvider, serverID, role string) (client *api.Client, token string, secret *api.Secret, err error) {

	// Create the Vault client.
	//
	// Configuration is gathered from environment variables by upstream vault package. Environment variables like
	// VAULT_ADDR and VAULT_SKIP_VERIFY are relevant. The VAULT_TOKEN environment variable shouldn't be needed.
	// https://www.vaultproject.io/docs/commands#environment-variables
	if client, err = api.NewClient(nil); err != nil {
		return nil, "", nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Acquire an AWS session.
	var sess *session.Session
	if sess, err = session.NewSession(); err != nil {
		return nil, "", nil, fmt.Errorf("failed to create AWS session: %w", err)
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
		return nil, "", nil, fmt.Errorf("failed to sign AWS identity request: %w", err)
	}

	// JSON marshal the headers.
	var headers []byte
	if headers, err = json.Marshal(request.HTTPRequest.Header); err != nil {
		return nil, "", nil, fmt.Errorf("failed to JSON marshal HTTP headers for AWS identity request: %w", err)
	}

	// Read the body of the request.
	var body []byte
	if body, err = ioutil.ReadAll(request.HTTPRequest.Body); err != nil {
		return nil, "", nil, fmt.Errorf("failed to JSON marshal HTTP body for AWS identity request: %w", err)
	}

	// Create the data to write to Vault.
	data := make(map[string]interface{})
	data["iam_http_request_method"] = request.HTTPRequest.Method
	data["iam_request_url"] = base64.StdEncoding.EncodeToString([]byte(request.HTTPRequest.URL.String()))
	data["iam_request_headers"] = base64.StdEncoding.EncodeToString(headers)
	data["iam_request_body"] = base64.StdEncoding.EncodeToString(body)
	data["role"] = role

	// Create the path to write to for Vault.
	//
	// The authProvider is the value referenced in the "Path" column in this documentation. It's likely "aws".
	// https://www.vaultproject.io/api/auth/aws#login
	path := fmt.Sprintf("auth/%s/login", authProvider)

	// Write the AWS token service request to Vault.
	if secret, err = client.Logical().Write(path, data); err != nil {
		return nil, "", nil, fmt.Errorf("failed to write data to Vault to get token: %w", err)
	}
	if secret == nil {
		return nil, "", nil, fmt.Errorf("failed to get token from Vault: %w", err)
	}

	// Get the Vault token from the response.
	if token, err = secret.TokenID(); err != nil {
		return nil, "", nil, fmt.Errorf("failed to get token from Vault response: %w", err)
	}

	// Set the token for the client as the one it just received.
	client.SetToken(token)

	return client, token, secret, nil
}

func GetSecret() (string, string, string) {
	config := vault.DefaultConfig()

	config.Address = "https://vault.dev.neocharge.io"

	client, token, whatIsThisSecret, err := AWSLogin("aws", "", "rolefromvault")
	if err != nil {
		log.Fatalf("unable to initialize Vault client: %v", err)
	}
	log.Println(token, whatIsThisSecret)

	secret, err := client.KVv1("aws").Get(context.Background(), "creds/sms_sender")
	if err != nil {
		log.Fatalf("unable to read secret: %v", err)
	}

	access_key, ok := secret.Data["access_key"].(string)
	if !ok {
		log.Fatalf("value type assertion failed: %T %#v", secret.Data["access_key"], secret.Data["access_key"])
	}

	secret_key, ok := secret.Data["secret_key"].(string)
	if !ok {
		log.Fatalf("value type assertion failed: %T %#v", secret.Data["secret_key"], secret.Data["secret_key"])
	}

	security_token, ok := secret.Data["security_token"].(string)
	if !ok {
		log.Fatalf("value type assertion failed: %T %#v", secret.Data["security_token"], secret.Data["security_token"])
	}

	return access_key, secret_key, security_token
}

func main() {

	randomNumber := strconv.Itoa(int(GenerateRandomNumber()))

	msgPtr := flag.String("m", randomNumber, "The message to send to the user")
	phoneNumber := flag.String("n", "+16366146678",
		"The phone number you want to send message to in E.164 format")

	flag.Parse()

	if *msgPtr == "" || *phoneNumber == "" {
		fmt.Println("You must supply a message and a phone number")
		os.Exit(1)
	}

	//TODO: auto renew vault credentials
	//TODO: auto renew aws credentials

	//get AWS credentials from vault
	access_key, secret_key, security_token := GetSecret()

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Region:      aws.String("us-west-2"),
			Credentials: credentials.NewStaticCredentials(access_key, secret_key, security_token),
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
