package main

import (
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
)

func PublishMessage(svc snsiface.SNSAPI, msg, phoneNumber *string) (*sns.PublishOutput, error) {
	result, err := svc.Publish(&sns.PublishInput{
		Message:     msg,
		PhoneNumber: phoneNumber,
	})

	return result, err
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

	client, err := VaultClient()
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
