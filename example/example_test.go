package raweml

import (
	"NewPortal/emailProcessor/raweml"
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ses"
	// "github.com/pborman/uuid"
)

// email body is wrapped afer 77 chars

func Example() {

	// parse Thread-Index value
	idx := "AdWrqyuNMGDKcPPKTE6qJN0A4Jd4nA=="
	emailThread := raweml.ParseEmailThread(idx, "some topic")
	resultThreadIndex := raweml.NewEmailThreadFromParams(emailThread.DateUnixNano, emailThread.GetGuid(), emailThread.GetTopic(), nil)
	fmt.Printf("MATCH Parsed and Generated Thread-Index: %v (%v == %v)\n", idx == resultThreadIndex.String(), idx, resultThreadIndex)

	// create New Thread-Index value
	idx = raweml.NewThread("Test Topic").String()
	idx = idx[:2] + "??????" + idx[2+6:] // mask the time
	fmt.Printf("New Thread-Index: %v", idx)

	// Output:
	//MATCH Parsed and Generated Thread-Index: true (AdWrqyuNMGDKcPPKTE6qJN0A4Jd4nA== == AdWrqyuNMGDKcPPKTE6qJN0A4Jd4nA==)
	//New Thread-Index: Ad??????WE0HYQEmX0q51rIrkL43iw==
}

func ExampleRaweml_send() {
	// simple email sent with AWS SES

	// build the email
	email := raweml.Email{
		From:     "MRI Diagnostics <no-reply@mrialerts.com>", // "sender@example.com",
		To:       "bjankulovski@mediaresources.com",
		Subject:  "Simple Test",
		HtmlBody: "<h1>Amazon SES Test Email (AWS SDK for Go)</h1><p><b>Time: </b>" + time.Now().Format("2006-01-02 15:04:05") + "</p>",
	}

	// send the email
	result, err := email.Send()

	// LogSendRawEmailResponse(err)

	// check the response
	validateOutput(email, result, err)

	// Output: ok
}

// func ExampleSendEmail() {
func ExampleRaweml_send_advanced() {
	// send email using AWS SES that contains:
	//	Thread-index	[Date, GUID(topic), Child Block]
	//  References 		topic
	//	Priority		[high, normal, low]
	//	from 	(multiple addresses)
	//	cc		(multiple addresses)
	//	bcc		(multiple addresses)
	//	EMAIL-FROM for AWS feedback ?????????????????
	//	Text body
	//	HTML body
	//	Attachment
	//	Embeded image ???????????????????????????????

	subject := "TESTING alerts: DEMO Panel 333  Heritage Dr. and 11 st. (Some Company Name) # 0000-0000A 0000-0000A 0000-0000A >>" + time.Now().Format("2006-01-02 15:04:05")
	topic := GetAlertTopic("525", "bose_user", raweml.GetNormilizedSubject(subject, 3))

	// build the email
	email := raweml.Email{
		"MRI Diagnostics <no-reply@mrialerts.com>", // "sender@example.com",
		"bjankulovski@mediaresources.com",
		"AWS Email Feedback <aws-feedback@mrialerts.com>",
		subject,
		"This email was sent with Amazon SES using the AWS SDK for Go.",
		"",
		// "<h1>Amazon SES Test Email (AWS SDK for Go)</h1><p>This email was sent with " +
		// 	"<a href='https://aws.amazon.com/ses/'>Amazon SES</a> using the " +
		// 	"<a href='https://aws.amazon.com/sdk-for-go/'>AWS SDK for Go</a>.</p>" +
		// 	"<br />" +
		// 	"<p><b>Time: </b>" + time.Now().Format("2006-01-02 15:04:05") + "</p>",
		"UTF-8",
		[]raweml.Attachment{{Name: "test.email"}},
		nil,
		raweml.High,
		topic, // when set,  "Thread-Topic", "Thread-Index" and "References" header attributs will be set
	}

	// send the email
	result, err := email.Send()

	// LogSendRawEmailResponse(err)

	// check the response
	validateOutput(email, result, err)

	// Output: ok
}

func LogSendRawEmailResponse(err error) {
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
func GetAlertTopic(displayId, username, subject string) string {
	today := raweml.TodayEST()
	alertType := strings.Split(subject, ":")[0]
	hashDisplayId := raweml.Hash(displayId)
	hashUser := raweml.Hash(username)

	buf := new(bytes.Buffer)
	buf.Write(hashDisplayId)
	buf.Write(hashUser)
	hashes := raweml.HexToBase64(buf.Bytes())

	return fmt.Sprintf("%s %s %s", today, alertType, hashes)
}
func validateOutput(email raweml.Email, result *ses.SendRawEmailOutput, err error) {
	if err == nil && strings.Contains(fmt.Sprintf("%v", result), "MessageId:") {
		fmt.Println("ok")
	} else {
		fmt.Println("Email Sent to address: " + email.To)
		fmt.Println("EMAIL FAILED: ", result, err)
	}

}
