package raweml

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------
// # TODO - create test for raweml
// ---------------------------------------------------------------
// - add test for `raweml` (test for GetSendRawEmailInput and NewRecipients)
// ---------------------------------------------------------------
var (
	testEmailString = `Content-Language: en-US
Content-Type: multipart/mixed; boundary=*
From: NO REPLAY EMAIL ACCOUNT <no-reply@example.com>
Mime-Version: 1.0
References: MbfJRQw5X+qg8GSOJxjM2Q==
Subject: Simple Test
Thread-Index: *
Thread-Topic: Hello world
To: customer@example.com
X-Priority: 3

*
Content-Type: multipart/alternative; boundary=*
Mime-Version: 1.0

*
Content-Transfer-Encoding: 7bit
Content-Type: text/plain; charset=UTF-8

Amazon SES Test Email (AWS SDK for Go)
*
Content-Transfer-Encoding: 7bit
Content-Type: text/html; charset=UTF-8

<h1>Amazon SES Test Email (AWS SDK for Go)</h1>
*
*
*
Content-Type: application/octet-stream
Content-Transfer-Encoding: base64
Content-ID: <1001>
X-Attachment-Id: 1001
Content-Disposition: attachment; filename="Mars.png"

*`
)

func TestRaweml(t *testing.T) {
	t.Run("Test conversion of Email to raw data", func(t *testing.T) {
		// create Email
		const fromEmail = "NO REPLAY EMAIL ACCOUNT <no-reply@example.com>"
		eml := Email{
			From:        fromEmail,
			Recipients:  NewRecipients("customer@example.com", "", ""),
			Subject:     "Simple Test",
			TextBody:    "Amazon SES Test Email (AWS SDK for Go)",
			HTMLBody:    "<h1>Amazon SES Test Email (AWS SDK for Go)</h1>",
			Topic:       "Hello world",
			Attachments: []Attachment{{Name: "example/Mars.png", ContentID: "1001"}},
			AwsRegion:   "us-east-1",
		}
		eml.SetHeader("X-something", "test")

		// get Email Raw data
		r, err := eml.GetSendRawEmailInput()
		if err != nil {
			t.Error(err)
		}

		// validate the email
		if got := eml.GetSource(); *got != fromEmail {
			t.Errorf("Invalid Source!\nwant:%s\ngot:%s", fromEmail, *got)
		}
		if want := "customer@example.com"; *r.Destinations[0] != want {
			t.Errorf("Invalid destination!\nwant:%v\ngot:%v", want, r.Destinations)
		}

		// line by line comparison
		str := strings.Split(strings.ReplaceAll(string(r.RawMessage.Data), "\r", ""), "\n")
		for i, line := range strings.Split(testEmailString, "\n") {
			if line != "*" {
				if strings.HasSuffix(line, "*") || strings.HasSuffix(line, "*\r") {
					// compare first part
					max := len(line) - 1
					if strings.HasSuffix(line, "*\r") {
						max = max - 1
					}
					if str[i][0:max] != line[0:max] {
						t.Errorf("Invalid RawMessage data line with wildchar! (%v)\nwant:%s\ngot:%s", i, line, str[i])
					}
				} else {
					if str[i] != line {
						t.Errorf("Invalid RawMessage data line! (%v)\nwant:%s\ngot:%s", i, line, str[i])
					}
				}
			}
		}
	})
	t.Run("Test New Recipients", func(t *testing.T) {
		to := "to_1@h.com,to_2@h.com"
		cc := "cc_1@h.com,c_2@h.com"
		bcc := "bcc_1@h.com,bcc_2@h.com"
		email := Email{
			Recipients: NewRecipients(to, cc, bcc),
		}

		// var want []*string
		// for _, v := range strings.Split(to+","+cc+","+bcc, ",") {
		// 	want = append(want, &v)
		// }
		want := to + "," + cc + "," + bcc
		got := email.Recipients.String()
		if want != got {
			t.Errorf("Invalid Recipients!\nwant:%s\ngot:%s", want, got)
		}
	})
}
