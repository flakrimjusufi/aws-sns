## A minimal CLI that interacts with AWS-SNS

Sends random 16 bytes numbers as SMS in phone numbers. 

Can be used for 2-FA or for sending random text messages. 

### How to run it?

1. Clone the repo in your local environment
2. Get your credentials **VAULT_TOKEN** from your vault-instance.
3. Run the application with one of the following commands:
~~~
VAULT_TOKEN=YOUR_VAULT_TOKEN \
go run main.go
~~~

If you want to specify a message you want to send in a specific number:
~~~
VAULT_TOKEN=YOUR_VAULT_TOKEN \
go run main.go -m "YOUR_MESSAGE" -n YOUR_PHONE_NUMBER
~~~