package raweml

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boseca/raweml"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ses"
)

// NOTE: email body is wrapped afer 77 chars

func Example() {

	// parse Thread-Index value
	idx := "AdWrqyuNMGDKcPPKTE6qJN0A4Jd4nA=="
	emailThread, err := raweml.ParseEmailThread(idx, "some topic")
	if err != nil {
		panic(err)
	}
	resultThreadIndex := raweml.NewEmailThreadFromParams(emailThread.DateUnixNano, emailThread.GetGUID(), emailThread.GetTopic(), nil)
	fmt.Printf("MATCH Parsed and Generated Thread-Index: %v (%v == %v)\n", idx == resultThreadIndex.String(), idx, resultThreadIndex)

	// create New Thread-Index value
	idx = raweml.NewThread("Test Topic").String()
	idx = idx[:2] + "??????" + idx[2+6:] // mask the time
	fmt.Printf("New Thread-Index: %v", idx)

	// Output:
	//MATCH Parsed and Generated Thread-Index: true (AdWrqyuNMGDKcPPKTE6qJN0A4Jd4nA== == AdWrqyuNMGDKcPPKTE6qJN0A4Jd4nA==)
	//New Thread-Index: Ad??????bwrdh8mmV5iaDFHXGlEfuQ==
}

// simple email sent with AWS SES
func Example_send() {

	// send the email
	err := raweml.Send(
		raweml.Email{
			From:       "NO REPLAY EMAIL ACCOUNT <no-reply@example.com>",
			Recipients: raweml.NewRecipients("customer@example.com", "", ""),
			Subject:    "Simple Test",
			HTMLBody:   "<h1>Amazon SES Test Email (AWS SDK for Go)</h1><p><b>Time: </b>" + time.Now().Format("2006-01-02 15:04:05") + "</p>",
			AwsRegion:  "us-east-1",
		})

	// check the response
	if err != nil {
		fmt.Printf("%v", err)
	} else {
		fmt.Print("ok")
	}

	// Output: ok
}

func Example_send_advanced() {
	// send email using AWS SES that contains:
	//	Thread-index	[Date, GUID(topic), Child Block]
	//  References 		topic
	//	Priority		[high, normal, low]
	//	from 	(multiple addresses)
	//	to		(multiple addresses)
	//	cc		(multiple addresses)
	//	bcc		(multiple addresses)
	//	Text body
	//	HTML body
	//	Attachment

	subject := "TEST email =?utf-8?B?5L2g5aW9?= >>" + time.Now().Format("2006-01-02 15:04:05")
	topic := getEmailTopic(525, "customer_username", getNormilizedSubject(subject, 3))

	// build the email
	email := raweml.Email{
		From:       "NO REPLAY EMAIL ACCOUNT <no-reply@example.com>",
		Recipients: raweml.NewRecipients("customer@example.com", "", ""),
		Feedback:   "", // "AWS Email Feedback <aws-feedback@example.com>",
		Subject:    subject,
		TextBody:   "This email was sent with Amazon SES using the AWS SDK for Go.",
		HTMLBody: "<h1>Amazon SES Test Email (AWS SDK for Go)</h1><p>This email was sent with " +
			"<a href='https://aws.amazon.com/ses/'>Amazon SES</a> using the " +
			"<a href='https://aws.amazon.com/sdk-for-go/'>AWS SDK for Go</a>.</p>" +
			"<img src='cid:1001' title='mars'/>" +
			"<br />" +
			"<p><b>Time: </b>" + time.Now().Format("2006-01-02 15:04:05") + "</p>",
		CharSet:     "UTF-8",
		Attachments: []raweml.Attachment{{Name: "Mars.png", ContentID: "1001"}}, //, ContentType: "image/png; name=\"Mars.png\""}},
		Headers:     nil,
		Priority:    raweml.PriorityHigh,
		Topic:       topic, // when set,  "Thread-Topic", "Thread-Index" and "References" header attributes will be set
		InReplyTo:   "",
		AwsRegion:   "us-east-1",
	}

	// send the email
	result, err := email.Send()

	// log the error
	logSendRawEmailResponse(err)

	// check the response
	validateOutput(email, result, err)

	// Output: ok
}

// Helping functions -----------------------

func logSendRawEmailResponse(err error) {
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ses.ErrCodeMessageRejected:
				fmt.Println(ses.ErrCodeMessageRejected, aerr.Error())
			case ses.ErrCodeMailFromDomainNotVerifiedException:
				fmt.Println(ses.ErrCodeMailFromDomainNotVerifiedException, aerr.Error())
			case ses.ErrCodeConfigurationSetDoesNotExistException:
				fmt.Println(ses.ErrCodeConfigurationSetDoesNotExistException, aerr.Error())
			case ses.ErrCodeConfigurationSetSendingPausedException:
				fmt.Println(ses.ErrCodeConfigurationSetSendingPausedException, aerr.Error())
			case ses.ErrCodeAccountSendingPausedException:
				fmt.Println(ses.ErrCodeAccountSendingPausedException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and Message from an error.
			fmt.Println(err.Error())
		}
		return
	}
}
func getEmailTopic(keyId int, username string, subject string) string {
	sKeyId := strconv.Itoa(keyId)
	emailType := strings.Split(subject, ":")[0]
	hashKeyId := hash(sKeyId)
	hashUser := hash(username)

	buf := new(bytes.Buffer)
	buf.Write(hashKeyId)
	buf.Write(hashUser)
	hashes := hexToBase64(buf.Bytes())

	return fmt.Sprintf("%s %s %s", sKeyId, emailType, hashes)
}
func validateOutput(email raweml.Email, result *ses.SendRawEmailOutput, err error) {
	if err == nil && strings.Contains(fmt.Sprintf("%v", result), "MessageId:") {
		fmt.Println("ok") // email successfully sent
	} else {
		fmt.Println("EMAIL FAILED: ", result, err)
	}
}
func getNormilizedSubject(subject string, level int) string {
	// return the subject without the "RE:" or "FW:" prefixes
	normalizedSubject := subject
	normalizedSubject = strings.TrimPrefix(normalizedSubject, "RE:")
	normalizedSubject = strings.TrimPrefix(normalizedSubject, "FW:")
	normalizedSubject = strings.TrimPrefix(normalizedSubject, " ")

	// do that again until we get rid of all prefixes
	if len(normalizedSubject) != len(subject) && level > 1 {
		normalizedSubject = getNormilizedSubject(normalizedSubject, level-1)
	}
	return normalizedSubject
}
func hexToBase64(bites []byte) string {
	return base64.StdEncoding.EncodeToString(bites)
}
func hash(s string) []byte {
	// create SHA1 hash for a given string
	h := sha1.New()
	h.Write([]byte(s))
	return h.Sum(nil)
}
