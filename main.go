package main

import (
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
	"os"
	"strconv"
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
	phoneNumber := flag.String("n", "[+][country code][area code][local phone number]",
		"The phone number you want to send message to in E.164 format")

	flag.Parse()

	if *msgPtr == "" || *phoneNumber == "" {
		fmt.Println("You must supply a message and a phone number")
		os.Exit(1)
	}

	// Initialize a session that the SDK will use to load
	// credentials from the shared credentials file. (~/.aws/credentials).
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
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
