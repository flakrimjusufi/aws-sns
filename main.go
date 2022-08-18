package main

import (
	"crypto/rand"
	"encoding/binary"
	"flag"
	"log"
	"strconv"
	"time"

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

	sess, err := NewAwsSession()
	if err != nil {
		log.Fatalln("Unable to create AWS session!", err)
	}

	log.Println("Vault client created.")
	log.Println("AWS session created.")
	log.Println("Running indefinitely...")

	for {
		log.Println("Checking credentials...")
		creds, err := sess.Config.Credentials.Get()
		if err != nil {
			log.Fatalln("Unable to get AWS session credentials!", err)
		}
		exp_time, err := sess.Config.Credentials.ExpiresAt()
		if err != nil {
			log.Fatalln("Unable to get AWS session credentials expiry time!", err)
		}
		log.Println("   Secret Access Key:", creds.SecretAccessKey)
		log.Println("             Expires:", exp_time)
		time.Sleep(1 * time.Minute)
	}
}

func GenerateRandomNumber() uint16 {
	var n uint16
	binary.Read(rand.Reader, binary.LittleEndian, &n)
	return n
}
