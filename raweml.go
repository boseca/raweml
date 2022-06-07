// Package raweml allows you to send emails with Priority and Conversation Topic using the AWS SES raw email.
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
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/google/uuid"
)

// Email is the structure containing all email details.
// To send the email just call the Send() method.
type Email struct {
	From        string
	Recipients  Recipients
	Feedback    string // feedback destination email address. If left blank "Return-path" or "From" address will be used instead.
	Subject     string // to change subject Charset use the MIME encoded-word syntax (e.g. "=?utf-8?B?5L2g5aW9?=") (ref: https://docs.aws.amazon.com/ses/latest/dg/send-email-raw.html)
	TextBody    string
	HTMLBody    string
	CharSet     string
	Attachments []Attachment // set it to `nil` if there are no attachments
	Headers     textproto.MIMEHeader
	Priority    EmailPriority
	Topic       string
	InReplyTo   string // Message-ID of the email to reply to in order for the email to be threaded. Gmail requires direct connection between emails to be threaded. Outlook is using Thread-Index and Thread-Topic instead
	AwsRegion   string // AWS Region of the SES service
}

// Recipients contains list of To, Cc, Bcc recipients
type Recipients struct {
	_            struct{}  `type:"structure"`
	BccAddresses []*string `type:"list"`
	CcAddresses  []*string `type:"list"`
	ToAddresses  []*string `type:"list"`
}

// Attachment represents an email attachment.
type Attachment struct {
	Name        string    // Name of the attachment
	Data        io.Reader // reader for the attachment. WARNING do not set this value to a nil *bytes.Buffer it will not be same as nil io.Reader and it will cause panic.
	FileName    string    // Name must be set to a valid fully qulified file name. If the FileName is set the Data reader will be ignored.
	ContentID   string    // Optional. Used for embedding images into the email (e.g. <img src="cid:{{ContentID}}">)
	ContentType string    // Optional. When blank falls back to 'application/octet-stream'.
}

// EmailPriority defines the type of priorty for the email
type EmailPriority string

// Email Priority Types
const (
	PriorityHigh   EmailPriority = "High"
	PriorityNormal EmailPriority = "Normal"
	PriorityLow    EmailPriority = "Low"
)

const crlf = "\r\n"

// Unique Application GUID used for defining the email conversation thread.
var (
	nameSpaceAppID = uuid.Must(uuid.Parse("9e01b615-a6a4-4883-b9bd-c1c80f4cceb4"))
)

// Send sends the email using the AWS SES
func Send(email Email) error {
	_, err := email.Send()
	return err
}

// NewRecipients converts comma separated list of to, cc and bcc into Recipients structure
func NewRecipients(to string, cc string, bcc string) (r Recipients) {
	if len(to) > 0 {
		for _, s := range strings.Split(to, ",") {
			r.ToAddresses = append(r.ToAddresses, aws.String(s))
		}
	}
	if len(cc) > 0 {
		for _, s := range strings.Split(cc, ",") {
			r.CcAddresses = append(r.CcAddresses, aws.String(s))
		}
	}
	if len(bcc) > 0 {
		for _, s := range strings.Split(bcc, ",") {
			r.BccAddresses = append(r.BccAddresses, aws.String(s))
		}
	}
	return r
}

// String converts Recipients structure to a string with comma separated recipients
func (r Recipients) String() string {
	return strings.Join(toStringArray(r.All()), ",")
}

// IsEmpty returns true if there are no recipients in any of To, Cc or Bcc
func (r Recipients) IsEmpty() bool {
	return len(r.ToAddresses) == 0 && len(r.CcAddresses) == 0 && len(r.BccAddresses) == 0
}

// All returns all recipients as an array of string pointers
func (r Recipients) All() []*string {
	return append(r.ToAddresses, append(r.CcAddresses, r.BccAddresses...)...)
}

// toStringArray converts array of string pointers to string array
func toStringArray(a []*string) []string {
	var r []string
	if len(a) > 0 {
		for _, o := range a {
			r = append(r, *o)
		}
	}
	return r
}

// Bcc returns a string of comma separated Bcc recipients
func (r Recipients) Bcc() string { return strings.Join(toStringArray(r.BccAddresses), ",") }

// Cc returns a string of comma separated Cc recipients
func (r Recipients) Cc() string { return strings.Join(toStringArray(r.CcAddresses), ",") }

// To returns a string of comma separated To recipients
func (r Recipients) To() string { return strings.Join(toStringArray(r.ToAddresses), ",") }

// Send sends the email
func (email Email) Send() (*ses.SendRawEmailOutput, error) {
	// create session
	svc := ses.New(session.New(&aws.Config{
		Region: aws.String(email.AwsRegion),
	}))
	// send email
	return email.SendWithSession(svc, nil)
}

// SendWithSession sends the email using provided svc session
func (email Email) SendWithSession(svc *ses.SES, input *ses.SendRawEmailInput) (result *ses.SendRawEmailOutput, err error) {
	if svc == nil {
		return nil, errors.New("Missing session parameter for SendWithInput function!")
	}
	if input == nil {
		if input, err = email.GetSendRawEmailInput(); err != nil {
			return nil, err
		}
	}
	return svc.SendRawEmail(input)
}

// GetSendRawEmailInput converts the email to *ses.SendRawEmailInput structure required by ses.SendRawEmail() method
func (email Email) GetSendRawEmailInput() (*ses.SendRawEmailInput, error) {

	// get whole email content as bytes
	emailBytes, err := email.Bytes()
	if err != nil {
		return nil, err
	}

	// return SendRawEmailInput
	return &ses.SendRawEmailInput{
		// Source:       email.GetSource(),	// commented out to send feedback email the same way as SendEmail
		Destinations: email.Recipients.All(),
		RawMessage: &ses.RawMessage{
			Data: emailBytes,
		},
	}, nil
}

// Bytes converts the email structure into email raw data bytes
func (email Email) Bytes() ([]byte, error) {
	// figure out the email parts
	hasAttachment := len(email.Attachments) > 0
	hasTxt := len(email.TextBody) > 0
	hasHTML := len(email.HTMLBody) > 0
	hasAlternative := hasTxt && hasHTML

	// validate the email
	if !(hasAttachment || hasTxt || hasHTML || hasAlternative) {
		return nil, errors.New("Cannot send empty email")
	}
	if email.Recipients.IsEmpty() {
		return nil, errors.New("At least one of the TO, CC  and BCC is required to send email.")
	}

	buf := new(bytes.Buffer)
	var writer *multipart.Writer

	// set Header attributes
	h := email.GetHeaders()

	setIfMissing(h, "From", email.From)
	setIfMissing(h, "To", email.Recipients.To())
	setIfMissing(h, "Cc", email.Recipients.Cc())
	setIfMissing(h, "Bcc", email.Recipients.Bcc())
	setIfMissing(h, "Return-Path", email.Feedback)
	setIfMissing(h, "Subject", email.Subject)

	// add Thread-Index
	if len(email.Topic) > 0 {
		thread := NewThread(email.Topic)
		setIfMissing(h, "Thread-Topic", thread.GetTopic())
		setIfMissing(h, "Thread-Index", thread.String())
		setIfMissing(h, "References", thread.Reference())
	}
	if len(email.InReplyTo) > 0 {
		setIfMissing(h, "In-Reply-To", email.InReplyTo)
	}

	// add Email Priority
	if email.Priority != PriorityNormal {
		setIfMissing(h, "Importance", email.Priority.String())
		setIfMissing(h, "X-Priority", email.Priority.ToNumber())
		// h.Set("X-MSMail-Priority", email.Priority.String())
	}

	// add language
	setIfMissing(h, "Content-Language", "en-US")

	// add multipart
	if hasAttachment {
		writer = multipart.NewWriter(buf)
		defer writer.Close() // this will not write the boundery because buffer is all ready flushed
		h.Set("Content-Type", "multipart/mixed; boundary=\""+writer.Boundary()+"\"")
	} else if hasAlternative {
		writer = multipart.NewWriter(buf)
		defer writer.Close()
		h.Set("Content-Type", "multipart/alternative; boundary=\""+writer.Boundary()+"\"")
	} else if hasTxt {
		h.Set("Content-Type", "text/plain; charset="+email.getCharSet()) // us-ascii
	} else if hasHTML {
		h.Set("Content-Type", "text/html; charset="+email.getCharSet()) // UTF-8
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
		if err := addPart(altWriter, "text/plain; charset="+email.getCharSet(), email.TextBody); err != nil {
			return nil, err
		}

		// HTML body:
		if err := addPart(altWriter, "text/html; charset="+email.getCharSet(), email.HTMLBody); err != nil {
			return nil, err
		}
		altWriter.Close()

	} else if hasAlternative || hasAttachment {
		// TEXT body
		if hasTxt {
			if err := addPart(writer, "text/plain; charset="+email.getCharSet(), email.TextBody); err != nil {
				return nil, err
			}
		}

		// HTML body:
		if hasHTML {
			if err := addPart(writer, "text/html; charset="+email.getCharSet(), email.HTMLBody); err != nil {
				return nil, err
			}
		}
	} else {
		if hasTxt {
			buf.Write([]byte(email.TextBody))
			fmt.Fprint(buf, crlf)
		} else if hasHTML {
			buf.Write([]byte(email.HTMLBody))
			fmt.Fprint(buf, crlf)
		} else {
			return nil, errors.New("Email is empty!")
		}
	}

	// Attachments (if there is any)
	if hasAttachment {
		if err := addAttachments(buf, email.Attachments, writer.Boundary()); err != nil {
			return nil, err
		}
	}

	// done writing
	if writer != nil {
		err := writer.Close()
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// GetHeaders returns a pointer to the email.Headers field
func (email Email) GetHeaders() *textproto.MIMEHeader {
	if email.Headers == nil {
		email.Headers = make(textproto.MIMEHeader)
	}
	return &email.Headers
}

// SetHeader sets the header entries associated with key to the single element value. It replaces any existing values associated with key.
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

func _addAttachment(w io.Writer, item Attachment, boundary string) error {
	contentType := item.ContentType
	if len(contentType) == 0 {
		contentType = "application/octet-stream"
	}
	fileReader := item.Data

	if fileReader == nil || fileReader == (*bytes.Buffer)(nil) || fileReader == (*os.File)(nil) {
		if len(item.FileName) > 0 {
			file, err := os.Open(item.FileName)
			if err != nil {
				return err
				// alternative: attach blank file
				// fmt.Fprintf(w, "\n--%s\n", boundary)
				// fmt.Fprintf(w, "Content-Type: text/plain; charset=utf-8\n")
				// fmt.Fprintf(w, "could not open file: %v\n", err)
			}
			fileReader = file
			defer file.Close()
		} else {
			return errors.New("Attachment Data and FileName are missing. At least one of them is required.")
		}
	}

	fmt.Fprintf(w, "\n--%s\n", boundary)
	fmt.Fprintf(w, "Content-Type: %s\n", contentType)
	fmt.Fprintf(w, "Content-Transfer-Encoding: base64\n")
	fmt.Fprintf(w, "Content-ID: <%s>\n", item.ContentID)
	fmt.Fprintf(w, "X-Attachment-Id: %s\n", item.ContentID)
	fmt.Fprintf(w, "Content-Disposition: attachment; filename=\"%s\"\n\n", filepath.Base(item.Name))

	b64 := base64.NewEncoder(base64.StdEncoding, w)
	defer b64.Close()

	if _, err := io.Copy(b64, fileReader); err != nil {
		return err
	}

	// compress
	// gzip := gzip.NewWriter(b64)
	// defer gzip.Close()
	// io.Copy(gzip, file)

	return nil
}

func addAttachments(w io.Writer, attachments []Attachment, boundary string) error {
	for _, item := range attachments {
		if err := _addAttachment(w, item, boundary); err != nil {
			return err
		}
	}
	return nil
}

// writeHeader writes the specified MIMEHeader to the io.Writer.
// Header values will be trimmed but otherwise left alone.
// Headers with multiple values are not supported and will return an error.
func writeHeader(w io.Writer, header *textproto.MIMEHeader) error {
	// for k, vs := range *header {
	for _, k := range sortedHeaders(header) {
		vs := header.Values(k)
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
	if len(value) > 0 {
		if _, ok := (*h)[key]; h != nil && !ok {
			h.Set(key, value)
		}
	}
}

// -- Helpter functions -------------------------------------------

// GetSource returns the From email address
func (email Email) GetSource() *string {
	if len(email.From) > 0 {
		return aws.String(email.From)
	}
	return nil
}

func (email Email) getCharSet() string {
	if len(email.CharSet) > 0 {
		return email.CharSet
	}
	return "UTF-8"
}

// ToNumber converts email priority to a string number
func (priority EmailPriority) ToNumber() string {
	switch priority {
	case PriorityHigh:
		return "1" // 1 - High
	case PriorityNormal:
		return "3" // 3 - Normal (default)
	case PriorityLow:
		return "5" // 5 - Low
	default:
		return "3" // 3 - Normal
	}
}

// String converts email priority to string
func (priority EmailPriority) String() string {
	return string(priority)
}

func sortedHeaders(header *textproto.MIMEHeader) (keys []string) {
	// type MIMEHeader map[string][]string
	for k := range *header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
