package raweml

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"testing"

	"github.com/google/uuid"
)

type TestItem struct {
	ThreadIndex string
	ThreadItem  Thread
	Result      []string
}
type TestConversionItem struct {
	FileTimeStamp uint64
	Expected      uint64
	Description   string
}

//gocyclo:ignore
func TestThread(t *testing.T) {

	// t.Skip("Skipping ALL thread tests!")

	testItems := []TestItem{

		// -- Single Email Threads  --------------------------------------------
		// From: John Doe <johndoe@example.com>
		// To: John Doe <johndoe@example.com>
		// Subject: Test important msg from Outlook
		// Thread-Topic: Test important msg from Outlook
		// Thread-Index: AdWtmt9I3YwkFRbJRzGIKv+YqcmJ2Q==
		// Importance: high
		// X-Priority: 1
		// Date: Sun, 8 Dec 2019 07:43:55 +0000
		//
		// From: John Doe <johndoe@example.com>
		// To: Jane Doe <janedoe@example.com>, John Smith
		// 	<johnsmith@example.com>, Richard Roe <richardroe@example.com>
		// Subject: Brightness capped to 40%
		// Thread-Topic: Brightness capped to 40%
		// Thread-Index: AdWrqyuNMGDKcPPKTE6qJN0A4Jd4nA==
		// Date: Thu, 5 Dec 2019 20:34:56 +0000

		{"AdWtmt9I3YwkFRbJRzGIKv+YqcmJ2Q==", NewEmailThreadFromParams(1575790875990425600, parseGUID("DD 8C 24 15 16 C9 47 31 88 2A FF 98 A9 C9 89 D9"), "", nil), nil}, // Sun, 8 Dec 2019 07:43:55 +0000
		{"AdWrqyuNMGDKcPPKTE6qJN0A4Jd4nA==", NewEmailThreadFromParams(1575577973571584000, parseGUID("30 60 CA 70 F3 CA 4C 4E AA 24 DD 00 E0 97 78 9C"), "", nil), nil}, // Thu, 5 Dec 2019 20:34:56 +0000
		{"AdWveZF6CBnh8oAcRyegkpj90Sd7ow==", NewEmailThreadFromParams(1575996474389299200, parseGUID("08 19 E1 F2 80 1C 47 27 A0 92 98 FD D1 27 7B A3"), "", nil), nil}, // 2019-12-10 16:47:54.3892992 +0000 UTC

		// -- Email threads With Child blocks --------------------------------------------
		// Ref: https://www.meridiandiscovery.com/how-to/e-mail-conversation-index-metadata-computer-forensics/
		// Base64:	Ac3pCr/g148OQoCCQSCy8dDjwH7QBwAAzLowAAARRGA=
		// HEX: 	01CDE90ABFE0 D78F0E4280824120B2F1D0E3C07ED007 0000CCBA30 0000114460
		// GUID: 	d78f0e42-8082-4120-b2f1-d0e3c07ed007
		// Date:  	January 2, 2013 17:01:04 (UTC)
		//	Diff 1:  22min 	53.897 sec	 	 January 2, 2013 17:23:58 (UTC)
		//	Diff 1:  1min 	55.868 sec	 	 January 2, 2013 17:23:58 (UTC)
		// -------------------------------------------------
		// From: John Doe <johndoe@example.com>
		// To: Customer Name <customer@example.com>
		// Subject: Test conversation
		// Thread-Topic: Test conversation
		// Thread-Index: AdWzEsgtBcdhxsJwRHGxWvOvVVjQCw==
		//				 01D5B312C82D 05C761C6C2704471B15AF3AF5558D00B
		// Date: Sun, 15 Dec 2019 06:42:19 +0000
		// .................................................
		// From: Customer Name <customer@example.com>
		// To: John Doe <johndoe@example.com>
		// Subject: RE: Test conversation
		// Thread-Topic: Test conversation
		// Thread-Index: AdWzEsgtBcdhxsJwRHGxWvOvVVjQCwAAAmpQ
		//				 01D5B312C82D 05C761C6C2704471B15AF3AF5558D00B 0000026A50
		// Date: Sun, 15 Dec 2019 06:42:45 +0000 	(26s)
		// .................................................
		// From: John Doe <johndoe@example.com>
		// To: Customer Name <customer@example.com>
		// Subject: RE: Test conversation
		// Thread-Topic: Test conversation
		// Thread-Index: AdWzEsgtBcdhxsJwRHGxWvOvVVjQCwAAAmpQAABnRrA=
		//				 01D5B312C82D 05C761C6C2704471B15AF3AF5558D00B 0000026A50 00006746B0
		//															162004992 5 0 6930563072 11 0
		// Date: Sun, 15 Dec 2019 06:54:12 +0000	(26s	11m 33s)
		// -------------------------------------------------------------------------------

		{"Ac3pCr/g148OQoCCQSCy8dDjwH7QBwAAzLowAAARRGA=", NewEmailThreadFromParams(
			int64(timeStampToUnix(130016196641685504)),
			parseGUID("d78f0e42-8082-4120-b2f1-d0e3c07ed007"),
			"",
			[]ChildBlock{
				{false, 13738967040 * 100, 3, 0}, // 0000CCBA30
				{false, 1158676480 * 100, 6, 0},  // 0000114460
			}), nil}, // January 2, 2013 17:01:04 (UTC)

		// ----- Test thread emails ---------------
		{"AdWzEsgtBcdhxsJwRHGxWvOvVVjQCw==", NewEmailThreadFromParams(
			int64(timeStampToUnix(132208657326473216)),
			parseGUID("05C761C6C2704471B15AF3AF5558D00B"),
			"",
			nil,
		), nil},
		{"AdWzEsgtBcdhxsJwRHGxWvOvVVjQCwAAAmpQ", NewEmailThreadFromParams(
			int64(timeStampToUnix(132208657326473216)),
			parseGUID("05C761C6C2704471B15AF3AF5558D00B"),
			"",
			[]ChildBlock{
				{false, 162004992 * 100, 5, 0},
			}), nil},
		{"AdWzEsgtBcdhxsJwRHGxWvOvVVjQCwAAAmpQAABnRrA=", NewEmailThreadFromParams(
			int64(timeStampToUnix(132208657326473216)),
			parseGUID("05C761C6C2704471B15AF3AF5558D00B"),
			"",
			[]ChildBlock{
				{false, 162004992 * 100, 5, 0},
				{false, 6930563072 * 100, 11, 0},
			}), nil},
	}
	testChildBlocks := map[string]ChildBlock{
		"0000CCBA30": {false, 1373896704000, 3, 0}, // 22m 53.897s
		"0000114460": {false, 115867648000, 6, 0},  // 1m 55.868

		"0000026A50": {false, 16200499200, 5, 0},   // 16m 30.2s
		"00006746B0": {false, 693056307200, 11, 0}, // 1h 59m
	}

	t.Run("Test Conversation File Time Stamp to Unix Time", func(t *testing.T) {
		// t.Skip("Skip: Test Conversation File Time Stamp to Unix Time")

		tests := map[int]TestConversionItem{
			1: {128166372003061629, 1172163600000000000, "2007-02-22 17:00:00.306162"},
			2: {128166372016382155, 1172163601000000000, "2007-02-22 17:00:01.638215"},
			3: {128166372026382245, 1172163602000000000, "2007-02-22 17:00:02.638224"},

			4: {130016196641685504, 1357146064000000000, "2013-01-02 17:01:04.000000"},
		}

		// one_sec := 1 * time.Second
		for _, item := range tests {
			got := timeStampToUnix(item.FileTimeStamp)
			delta := time.Duration(deltaUint64(got, item.Expected))
			// fmt.Printf("delta diff: %v - %v\n", item.FileTimeStamp, delta)
			if delta.Seconds() >= 1 {
				t.Error(fmt.Sprintf("Time Conversion missmatch (delta %v)! Expected/Got: \n%v / %v  (%v/%v)\n", delta, item.Expected, got, item.Description, time.Unix(0, int64(got)).UTC()))
			}
		}
	})
	t.Run("Test Parsing Child Block", func(t *testing.T) {
		// t.Skip("Skip: Test Parsing Child Block")

		msg := ""
		for key, item := range testChildBlocks {
			parsed, _ := ParseChildBlock(hexToString(key))
			msg += matchChildBlock(parsed, item)
		}
		if len(msg) > 0 {
			t.Error(fmt.Sprintf("Child Block missmatch:\n%v", msg))
		}
	})
	t.Run("Test Converting Child Block to String", func(t *testing.T) {
		// t.Skip("Skip: Test Converting Child Block to String")

		msg := ""
		for key, item := range testChildBlocks {
			got := stringToHex(item.String())
			if key != got {
				msg += fmt.Sprintf("String missmatch! Expected/got %v / %v\n", key, got)
			}
		}
		if len(msg) > 0 {
			t.Error(fmt.Sprintf("Child Block missmatch:\n%v", msg))
		}
	})
	t.Run("Test Creating Child Block", func(t *testing.T) {
		// t.Skip("Skip: Test Creating Child Block")

		msg := ""
		for key, item := range testChildBlocks {
			got := NewChildBlock(item.TimeDifference)
			got.RandomNum = item.RandomNum
			got.SequenceCount = item.SequenceCount
			// fmt.Printf("Matching child blocks: %v %X %v %v\n", key, got.String(), time.Duration(item.TimeDifference)*time.Nanosecond, time.Duration(got.TimeDifference)*time.Nanosecond)
			if key != stringToHex(got.String()) {
				msg += fmt.Sprintf("String missmatch! Expected/got %v / %X\n", key, got)
			}
		}
		if len(msg) > 0 {
			t.Error(fmt.Sprintf("Child Block missmatch:\n%v", msg))
		}
	})
	t.Run("Test Parsing Thread-Index", func(t *testing.T) {
		// t.Skip("Skip: Test Parsing Thread-Index")

		// check test items
		for i := 0; i < len(testItems); i++ {
			// var item TestItem
			item := testItems[i]

			// match parsed with defined
			parsed, err := ParseEmailThread(item.ThreadIndex, "")
			if err != nil {
				panic(err)
			}
			r := matchEmailThread(parsed, item.ThreadItem)
			// fmt.Printf("Matching: '%v'  '%v'\n", item.ThreadIndex, parsed.String())
			if len(r) > 0 {
				item.Result = append(item.Result, r)
			}

			// validate
			if len(item.Result) > 0 {
				t.Error(fmt.Sprintf("Match failed '%v'\n: '%v'\n", item.ThreadIndex, item.Result))
			} else {
				// fmt.Printf("Matched: %v\n", item.ThreadIndex)
			}
		}
	})
	t.Run("Test Creating Thread-Index", func(t *testing.T) {
		// t.Skip("Skip: Test Creating Thread-Index")

		// check test items
		for i := 0; i < len(testItems); i++ {
			var item TestItem
			item = testItems[i]

			// match created with defined
			created := NewEmailThreadFromParams(item.ThreadItem.DateUnixNano, item.ThreadItem.GetGUID(), item.ThreadItem.GetTopic(), cloneChildBlock(item.ThreadItem.ChildBlocks))
			r := matchEmailThread(created, item.ThreadItem)
			if len(r) > 0 {
				item.Result = append(item.Result, r)
			}

			// validate
			if len(item.Result) > 0 {
				t.Error(fmt.Sprintf("Match failed '%v'\n: '%v'\n", item.ThreadIndex, item.Result))
			} else {
				// fmt.Printf("Matched: %v\n", item.ThreadIndex)
			}
		}
	})
	t.Run("Test Parsing and Creating Thread-Index", func(t *testing.T) {
		// t.Skip("Skip: Test Parsing and Creating Thread-Index")

		// check test items
		for i := 0; i < len(testItems); i++ {
			var item TestItem
			item = testItems[i]

			// match created with parsed
			parsed, err := ParseEmailThread(item.ThreadIndex, "")
			if err != nil {
				panic(err)
			}
			created := NewEmailThreadFromParams(item.ThreadItem.DateUnixNano, item.ThreadItem.GetGUID(), item.ThreadItem.GetTopic(), cloneChildBlock(item.ThreadItem.ChildBlocks))
			r := matchEmailThread(created, parsed)
			if len(r) > 0 {
				item.Result = append(item.Result, r)
			}

			// validate
			if len(item.Result) > 0 {
				t.Error(fmt.Sprintf("Match failed '%v'\n: '%v'\n", item.ThreadIndex, item.Result))
			}
		}
	})
}

// helping functions -----------------------

func cloneChildBlock(c []ChildBlock) []ChildBlock {
	r := []ChildBlock{}
	for i := 0; i < len(c); i++ {
		parsed, err := ParseChildBlock(c[i].String())
		if err != nil {
			panic(err)
		}
		r = append(r, parsed)
	}
	return r
}
func matchEmailThread(src Thread, dest Thread) string {
	// match each fields
	msg := ""
	if src.DateUnixNano != dest.DateUnixNano {
		msg += fmt.Sprintf("DateUnixNano missmatch! got %v expected %v\n", src.DateUnixNano, dest.DateUnixNano)
	}
	if src.GetGUID().String() != dest.GetGUID().String() {
		msg += fmt.Sprintf("GUID missmatch! got %v expected %v\n", src.GetGUID(), dest.GetGUID())
	}
	if src.GetTopic() != dest.GetTopic() {
		msg += fmt.Sprintf("Topic missmatch! got %v expected %v\n", src.GetTopic(), dest.GetTopic())
	}
	if len(src.ChildBlocks) != len(dest.ChildBlocks) {
		msg += fmt.Sprintf("ChildBlocks missmatch! got %v expected %v\n", src.ChildBlocks, dest.ChildBlocks)
	} else if len(src.ChildBlocks) > 0 {
		for i := 0; i < len(src.ChildBlocks); i++ {
			msg += matchChildBlock(src.ChildBlocks[i], dest.ChildBlocks[i])
		}
	}
	if src.String() != dest.String() {
		msg += fmt.Sprintf("String missmatch! got %v expected %v\n", src.String(), dest.String())
	}

	return msg
}
func matchChildBlock(src ChildBlock, dest ChildBlock) string {
	// fmt.Printf("diff: %v (%v) \n", dest.TimeDifference, time.Duration(dest.TimeDifference)*time.Nanosecond)
	msg := ""
	if src.TimeFlag != dest.TimeFlag {
		msg += fmt.Sprintf("ChildBlock TimeFlag missmatch! got %v expected %v\n", src.TimeFlag, dest.TimeFlag)
	}
	if src.TimeDifference != dest.TimeDifference {
		msg += fmt.Sprintf("ChildBlock TimeDifference missmatch! got %v expected %v\n", src.TimeDifference, dest.TimeDifference)
	}
	if src.RandomNum != dest.RandomNum {
		msg += fmt.Sprintf("ChildBlock RandomNum missmatch! got %v expected %v\n", src.RandomNum, dest.RandomNum)
	}
	if src.SequenceCount != dest.SequenceCount {
		msg += fmt.Sprintf("ChildBlock SequenceCount missmatch! got %v expected %v\n", src.SequenceCount, dest.SequenceCount)
	}
	if src.String() != dest.String() {
		msg += fmt.Sprintf("ChildBlock String missmatch! got %X expected %X\n", src.String(), dest.String())
	}
	return msg
}
func hexToString(hexStr string) string {
	// Example:
	//	hexToString("0000CCBA30")

	org, err := hex.DecodeString(strings.Replace(hexStr, " ", "", -1))
	if err != nil {
		panic(err)
	}
	s := string(org)

	// fmt.Printf("%v\n", s)
	return s
}
func stringToHex(s string) string {
	return fmt.Sprintf("%X", s)
}
func hexToBase64Str(hexStr string) string {
	// Example:
	//	hexToBase64Str("01CDE90ABFE0D78F0E4280824120B2F1D0E3C07ED0070000CCBA300000114460")

	org, _ := hex.DecodeString(strings.Replace(hexStr, " ", "", -1))
	b64 := base64.StdEncoding.EncodeToString(org)

	fmt.Printf("%v\n", b64)
	return b64
}
func base64ToString(s string) string {
	// Example:
	//	base64ToString("AdWurZGyYp7iwi8YQiqGubZnWGTREQAGvWWwAAJBp8AAWldBEAAClz1AAADdB2AAAH9D0AADljsQAABWWAAAAZIZEAAll1IgAAAaP/AABV+kAA==")
	bytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}
func stringToBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
func deltaUint64(a uint64, b uint64) uint64 {
	if a >= b {
		return a - b
	}
	return b - a
}
func getChildBlock(s string) ChildBlock {
	r, _ := ParseChildBlock(s)
	return r
}
func parseGUID(s string) uuid.UUID {
	// remove dashes/spaces + decode string to bytes[]
	s = strings.Replace(s, "-", "", -1)
	s = strings.Replace(s, " ", "", -1)
	guidBytes, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	// create UUID from bytes
	guid, err1 := uuid.FromBytes(guidBytes)
	if err1 != nil {
		panic(err1)
	}

	return guid
}

// / helping functions -----------------------
