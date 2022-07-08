## A minimal CLI that interacts with AWS-SNS

Sends random 16 bytes numbers as SMS in phone numbers. 

Can be used for 2-FA or for sending random text messages. 

### How to run it?

1. Clone the repo in your local environment
2. Get your credentials **AWS_ACCESS_KEY_ID**, **AWS_SECRET_ACCESS_KEY** and  **AWS_REGION** from your aws-instance.
3. Run the application with the following command:
~~~
AWS_ACCESS_KEY_ID=YOUR_AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY=YOUR_AWS_SECRET_ACCESS_KEY AWS_REGION=YOUR_AWS_REGION \
go run main.go
~~~

If you want to specify a message you want to send in a specific number:
~~~
AWS_ACCESS_KEY_ID=YOUR_AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY=YOUR_AWS_SECRET_ACCESS_KEY AWS_REGION=YOUR_AWS_REGION \
go run main.go -m "YOUR_MESSAGE" -n YOUR_PHONE_NUMBER
~~~