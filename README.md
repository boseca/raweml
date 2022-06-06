RawEml
=========
This package is for a granual control of emails that are sent with AWS SES.  
It supports setting the email priority, conversation topic for grouping and any other email header tags.

[![Build Status](https://github.com/boseca/raweml/workflows/build/badge.svg)](https://github.com/boseca/raweml/actions?query=workflow%3Abuild)
[![Coverage Status](https://coveralls.io/repos/github/boseca/raweml/badge.svg?branch=master)](https://coveralls.io/github/boseca/raweml?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/boseca/raweml)](https://goreportcard.com/report/github.com/boseca/raweml)
[![Go Reference](https://pkg.go.dev/badge/github.com/boseca/raweml.svg)](https://pkg.go.dev/github.com/boseca/raweml)

## Description

RawEml package allows you to specify any header attributes for an email sent with AWS SES 

Supported email attributes:
- priority 
- thread-topic (works for Outlook but not for Gmail)
- any additional attributes via `Headers` field

>NOTE: Attributes defined in `Headers` field have precedence over any other fields.  
Example: If `From` header attribute is defined in the `Headers` field then the value in `email.From` will be ignored

Following fields can be used to build the email struct
- From      (multiple addresses)
- Recipients
    - to		(multiple addresses)
    - cc		(multiple addresses)
    - bcc		(multiple addresses)
- Feedback      (feedback address)
- Subject
- Text body
- HTML body
- CharSet
- Attachment
- Headers       (email header attributes)
- Priority		[high, normal, low]
- Topic
    - Thread-index	[Date, GUID(topic), Child Block]
    - References 		topic
- InReplyTo     (Message-ID of the email to reply to in order for the email to be threaded. Gmail requires direct connection between emails to be threaded. Outlook is using Thread-Index and Thread-Topic instead)
- AwsRegion     (AWS SES region. Example `us-east-1`)

## Download

`go get gopkg.in/raweml`

## Usage

To send the email call `raweml.Send(email)` method.  


## Examples

See the [examples in the documentation](https://godoc.org/gopkg.in/raweml#example-package).


## Build

```bash
go build
```

## Test

- Run all tests
```bash
go test -v
```

- Run single test
```bash
go test -v -run ^TestThread/Test_Parsing_Thread-Index$ github.com/boseca/raweml
```

- Run examples
>Make sure to change the sender email address to an AWS verified email address before running the examples
```bash
go test -v ./example
```
- Lint
```bash
golint
```

- Show code coverage
```bash
go test -coverprofile=c.out
sed -i "s/$(pwd|sed 's/\//\\\//g')/./g" c.out   # convert absolute path to relative path 
go tool cover -html=c.out -o=c.html             # optional
gcov2lcov -infile=c.out -outfile=c.lcov
genhtml -q --legend -o coverage_html --title='Raweml' c.lcov
x-www-browser file:///$(pwd)/coverage_html/index.html
```
