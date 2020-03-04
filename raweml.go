package raweml

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/google/uuid"
)

type Email struct {
	From        string
	To          string
	Feedback    string // feedback destination. If left blank "Return-path" or "From" address will be used instead
	Subject     string
	TextBody    string
	HtmlBody    string
	CharSet     string
	Attachments []Attachment // optional
	Headers     textproto.MIMEHeader
	Priority    EmailPriority
	Topic       string
}

// Attachment represents an email attachment.
type Attachment struct {
	// Name must be set to a valid file name.
	Name string

	// Optional.
	// Uses mime.TypeByExtension and falls back to application/octet-stream if unknown.
	ContentType string

	Data io.Reader
}

type EmailPriority string

const (
	High   EmailPriority = "High"
	Normal EmailPriority = "Normal"
	Low    EmailPriority = "Low"
)

const crlf = "\r\n"

var (
	NameSpaceAppId = uuid.Must(uuid.Parse("9e01b615-a6a4-4883-b9bd-c1c80f4cceb4"))
)

func (email Email) Send() (*ses.SendRawEmailOutput, error) {

	// get email Input
	input := email.GetSendRawEmailInput()

	// send email
	svc := ses.New(session.New(&aws.Config{
		Region: aws.String("us-east-1"),
	}))

	// return results
	// fmt.Printf("\nEmail:\n%v", string(input.RawMessage.Data))
	return svc.SendRawEmail(input)

}
func (email Email) SendWithSession(svc *ses.SES, input *ses.SendRawEmailInput) (result *ses.SendRawEmailOutput, err error) {
	if svc == nil {
		panic(errors.New("Missing session parameter for SendWithInput function!"))
	}
	if input == nil {
		input = email.GetSendRawEmailInput()
	}
	return svc.SendRawEmail(input)
}

func (email Email) GetSendRawEmailInput() *ses.SendRawEmailInput {

	// get whole email content as bytes
	emailBytes, err := email.Bytes()
	if err != nil {
		panic(err)
	}

	// return SendRawEmailInput
	return &ses.SendRawEmailInput{
		Destinations: []*string{aws.String(email.To)},
		Source:       email.GetSource(),
		RawMessage: &ses.RawMessage{
			Data: emailBytes,
		},
	}
}
func (email Email) GetSource() *string {
	if len(email.Feedback) > 0 {
		return aws.String(email.Feedback)
	} else {
		return nil
	}
}

func (email Email) Bytes() ([]byte, error) {
	// ref: https://github.com/jpoehls/gophermail/blob/master/main.go

	// figure out the email parts
	hasAttachment := len(email.Attachments) > 0
	hasTxt := len(email.TextBody) > 0
	hasHTML := len(email.HtmlBody) > 0
	hasAlternative := hasTxt && hasHTML

	// validate the email
	if !(hasAttachment || hasTxt || hasHTML || hasAlternative) {
		return nil, errors.New("Cannot send empty email")
	}
	if email.To == "" { // && email.CC && email.BCC
		return nil, errors.New("At least one of the TO, CC  and BCC is required to send email.")
	}

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)
	defer writer.Close()

	// set Header attributes
	h := email.GetHeaders()

	setIfMissing(h, "From", email.From)
	setIfMissing(h, "To", email.To)
	// setIfMissing(h,"Return-Path", sender)
	setIfMissing(h, "Subject", email.Subject)

	// add Thread-Index
	if len(email.Topic) > 0 {
		thread := NewThread(email.Topic)
		setIfMissing(h, "Thread-Topic", thread.GetTopic())
		setIfMissing(h, "Thread-Index", thread.String())
		setIfMissing(h, "References", thread.Reference())
	}

	// add Email Priority
	if email.Priority != Normal {
		setIfMissing(h, "Importance", email.Priority.String())
		setIfMissing(h, "X-Priority", email.Priority.ToNumber())
		// h.Set("X-MSMail-Priority", email.Priority.String())
	}

	// add language
	setIfMissing(h, "Content-Language", "en-US")

	// add multipart
	if hasAttachment {
		writer = multipart.NewWriter(buf)
		h.Set("Content-Type", "multipart/mixed; boundary=\""+writer.Boundary()+"\"")
	} else if hasAlternative {
		writer = multipart.NewWriter(buf)
		h.Set("Content-Type", "multipart/alternative; boundary=\""+writer.Boundary()+"\"")
	} else if hasTxt {
		h.Set("Content-Type", "text/plain; charset=us-ascii;")
	} else if hasHTML {
		h.Set("Content-Type", "text/html; charset=UTF-8")
	} else {
		return nil, errors.New("Missing email content!")
	}
	setIfMissing(h, "MIME-Version", "1.0")

	// write main Header
	writeHeader(buf, h)

	// - alternative
	if hasAlternative && hasAttachment {
		// Nested Alternative parts
		altWriter := multipart.NewWriter(buf)
		defer altWriter.Close()

		hAlt := make(textproto.MIMEHeader)
		hAlt.Set("Content-Type", "multipart/alternative; boundary=\""+altWriter.Boundary()+"\"")
		hAlt.Set("MIME-Version", "1.0")
		_, err := writer.CreatePart(hAlt)
		if err != nil {
			return nil, err
		}

		// TEXT body
		if err := addPart(altWriter, "text/plain; charset=us-ascii", email.TextBody); err != nil {
			return nil, err
		}

		// HTML body:
		if err := addPart(altWriter, "text/html; charset=UTF-8", email.HtmlBody); err != nil {
			return nil, err
		}
		altWriter.Close()

	} else if hasAlternative || hasAttachment {
		// TEXT body
		if hasTxt {
			if err := addPart(writer, "text/plain; charset=us-ascii", email.TextBody); err != nil {
				return nil, err
			}
		}

		// HTML body:
		if hasHTML {
			if err := addPart(writer, "text/html; charset=UTF-8", email.HtmlBody); err != nil {
				return nil, err
			}
		}
	} else {
		if hasTxt {
			buf.Write([]byte(email.TextBody))
			fmt.Fprint(buf, crlf)
		} else if hasHTML {
			buf.Write([]byte(email.HtmlBody))
			fmt.Fprint(buf, crlf)
		} else {
			return nil, errors.New("Email is empty!")
		}
	}

	// Attachments (if there is any)
	addAttachments(buf, email.Attachments, writer.Boundary())

	// done writing
	if writer != nil {
		err := writer.Close()
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (email Email) GetHeaders() *textproto.MIMEHeader {
	if email.Headers == nil {
		email.Headers = make(textproto.MIMEHeader)
	}
	return &email.Headers
}

// Set sets the header entries associated with key to the single element value. It replaces any existing values associated with key.
func (email *Email) SetHeader(key, value string) {
	h := email.GetHeaders()
	h.Set(key, value)
}

func addPart(writer *multipart.Writer, contentType string, body string) error {

	h := make(textproto.MIMEHeader)
	h.Set("Content-Type", contentType)
	h.Set("Content-Transfer-Encoding", "7bit")
	part, err := writer.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = part.Write([]byte(body))
	if err != nil {
		return err
	}
	return nil
}

func _addAttachment(w io.Writer, file, boundary string) {
	fmt.Fprintf(w, "\n--%s\n", boundary)
	contents, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(w, "Content-Type: text/plain; charset=utf-8\n")
		fmt.Fprintf(w, "could not open file: %v\n", err)
	} else {
		defer contents.Close()
		fmt.Fprintf(w, "Content-Type: application/octet-stream\n")
		fmt.Fprintf(w, "Content-Transfer-Encoding: base64\n")
		fmt.Fprintf(w, "Content-Disposition: attachment; filename=\"%s\"\n\n", filepath.Base(file))

		b64 := base64.NewEncoder(base64.StdEncoding, w)
		defer b64.Close()
		io.Copy(b64, contents)

		// compress
		// gzip := gzip.NewWriter(b64)
		// defer gzip.Close()
		// io.Copy(gzip, contents)

	}
}

func addAttachments(w io.Writer, attachments []Attachment, boundary string) error {
	path := "/c/Projects/go/src/NewPortal/emailProcessor/.idea/"
	for _, item := range attachments {
		_addAttachment(w, path+item.Name, boundary)
	}
	return nil
}

// writeHeader writes the specified MIMEHeader to the io.Writer.
// Header values will be trimmed but otherwise left alone.
// Headers with multiple values are not supported and will return an error.
func writeHeader(w io.Writer, header *textproto.MIMEHeader) error {
	for k, vs := range *header {
		_, err := fmt.Fprintf(w, "%s: ", k)
		if err != nil {
			return err
		}

		for i, v := range vs {
			v = textproto.TrimString(v)

			_, err := fmt.Fprintf(w, "%s", v)
			if err != nil {
				return err
			}

			if i < len(vs)-1 {
				return errors.New("Multiple header values are not supported.")
			}
		}

		_, err = fmt.Fprint(w, crlf)
		if err != nil {
			return err
		}
	}

	// Write a blank line as a spacer
	_, err := fmt.Fprint(w, crlf)
	if err != nil {
		return err
	}

	return nil
}
func setIfMissing(h *textproto.MIMEHeader, key, value string) {
	if _, ok := (*h)[key]; h != nil && !ok {
		h.Set(key, value)
	}
}

// -- Helpter functions -------------------------------------------

// TodayEST returns current date portion of time in EST timezone
func TodayEST() string {
	layout := "2006-01-02"
	loc, err := time.LoadLocation("EST")
	if err != nil {
		fmt.Println(err)
	}
	return time.Now().UTC().In(loc).Format(layout)
}

func (priority EmailPriority) ToNumber() string {
	switch priority {
	case High:
		return "1" // 1 - High
	case Normal:
		return "3" // 3 - Normal (default)
	case Low:
		return "5" // 5 - Low
	default:
		return "3" // 3 - Normal
	}
}
func (priority EmailPriority) String() string {
	return string(priority)
}

// func win32TimeFromTar(key string, hdrs map[string]string, unixTime time.Time) Filetime {
// 	if s, ok := hdrs[key]; ok {
// 		n, err := strconv.ParseUint(s, 10, 64)
// 		if err == nil {
// 			return Filetime{uint32(n & 0xffffffff), uint32(n >> 32)}
// 		}
// 	}
// 	return NsecToFiletime(unixTime.UnixNano())
// }

// -- /Helpter functions -------------------------------------------
