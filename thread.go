package raweml

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	// "github.com/pborman/uuid"
)

// Thread represents an email thread (conversation group)
type Thread struct {
	DateUnixNano int64        // Thread Date in Unix Nanoseconds
	guid         uuid.UUID    // created based on the "topic" NOTE: Once the thread is created guid is saved and cannot be changed
	topic        string       // usually a normalized subject (subject without prefixes "RE:", "FW:")
	ChildBlocks  []ChildBlock // Sub-Thread
}

// ChildBlock represents a sub thread of the email thread
type ChildBlock struct {
	// TimeFlag: 1 bit
	//		0 when TimeDiff < 0.02s && TimeDiff > 2 yearsy;
	//		1 when TimeDiff < 1s && TimeDiff > 56 years)
	TimeFlag bool

	// TimeDifference: time difference between the child block create time and the time in the header block expressed in FILETIME units
	// 		if TimeFlag = 0 : discard high 15 bits and low 18 bits
	// 		if TimeFlag = 1 : discard high 10 bits and low 32 bits
	TimeDifference int64 // Unix NanoSecond

	// RandomNum: random number gernerated by calling GetTickCount()
	RandomNum byte

	// SequenceCount: default set to 0 (Four bits containing a sequence count that is taken from part of the random number.)
	SequenceCount byte
}

func NewThread(topic string) Thread {
	data := Hash(topic)
	guid := uuid.NewSHA1(NameSpaceAppId, data)

	return Thread{
		time.Now().UTC().UnixNano(),
		guid,
		topic,
		nil,
	}
}
func NewEmailThreadFromParams(dateUnixNanoSec int64, guid uuid.UUID, topic string, childBlocks []ChildBlock) (r Thread) {

	return Thread{
		dateUnixNanoSec,
		guid,
		topic,
		childBlocks,
	}
}
func ParseEmailThread(idx string, topic string) (r Thread) {
	// Thread-Index is composed of 22 bytes total + 0 or more child blocks of 5 bytes
	//  1 byte	- reserved (value 1) (used with next 5 bytes as 6 bytes structure holding the FILETIME value)
	//  5 bytes	- (plus the first byte) current system time converted to the FILETIME structure format
	// 16 bytes - holiding GUID
	// -------------------------------------------------------------------------------------------------
	// ref: https://docs.microsoft.com/en-us/office/client-developer/outlook/mapi/tracking-conversations
	// -------------------------------------------------------------------------------------------------

	if len(idx) < 22 {
		panic("Inavlid Thread-Index. Expected minimum 22 bytes.")
	}

	// decode Base64
	bytes, errD := base64.StdEncoding.DecodeString(idx)
	if errD != nil {
		panic(errD)
	}

	// get TimeStamp (first 6 bytes)
	bTS := [8]byte{0, 0, 0, 0, 0, 0, 0, 0}
	copy(bTS[:6], bytes[:6])

	// convert TimeStamp to Unix nanoseconds
	uxNs := TimeStampToUnix(binary.BigEndian.Uint64(bTS[:]))

	// Unix Time in nanoseconds
	threadTimeUnixNano := time.Unix(0, int64(uxNs)).UTC().UnixNano()

	// GUID portion
	threadGuid, errG := uuid.FromBytes(bytes[6:22])
	if errG != nil {
		panic(errG)
	}

	// child blocks
	var childBlocks []ChildBlock
	for i := 22; i < len(bytes) && i < (22+500*5); i += 5 {
		block, err := ParseChildBlock(string(bytes[i : i+5]))
		if err != nil {
			panic(err)
		} else {
			childBlocks = append(childBlocks, block)
		}
	}

	return Thread{
		threadTimeUnixNano,
		threadGuid,
		topic,
		childBlocks,
	}

}
func (thread *Thread) AddChildBlock() {
	deltaTime := time.Since(time.Unix(0, thread.DateUnixNano))
	thread.ChildBlocks = append(thread.ChildBlocks, NewChildBlock(deltaTime.Nanoseconds()))
}
func (thread Thread) String() string {
	return string(thread.Bytes())
}
func (thread Thread) Bytes() (r []byte) {

	// get Unix nanoseconds
	tn := thread.DateUnixNano

	// conver to timestamp
	tn = UnixToTimeStamp64(tn)
	tsBytes := IntToBytes(tn)

	// compose Thread Index
	bufIdx := new(bytes.Buffer)
	encoder := base64.NewEncoder(base64.StdEncoding, bufIdx)
	defer encoder.Close()
	encoder.Write(tsBytes[:6])                     // 6  - TIME_STAMP
	encoder.Write(thread.GuidBytes())              // 16 - GUID
	for i := 0; i < len(thread.ChildBlocks); i++ { // 5  - per Child block
		encoder.Write(thread.ChildBlocks[i].Bytes())
	}

	encoder.Close()
	return bufIdx.Bytes()
}
func (thread Thread) GuidBytes() (bytes []byte) {
	bytes, err := thread.guid.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return bytes
}
func (thread Thread) Write(w io.Writer) {
	w.Write(thread.Bytes())
}
func (thread Thread) Index() string {
	return string(thread.Bytes())
}

// Reference returns a hashed version of the Thread GUID
func (thread *Thread) Reference() string {
	return HexToBase64(thread.GuidBytes())
}
func (thread Thread) GetGuid() uuid.UUID {
	return thread.guid
}
func (thread Thread) GetTopic() string {
	return thread.topic
}

func NewChildBlock(deltaTimeUxNs int64) (r ChildBlock) {
	// child block is composed of 5 bytes total as follows:
	// 1 bit 	- One  bit containing a code representing the difference between the current time and the time stored in the header block.
	//				This bit will be: 0 if the difference is less than .02 second and greater than two years and
	//								  1 if the difference is less than one second and greater than 56 years.
	// 				   default value: 0 although MS doesn't specify what value should be set between 1s and 2y
	// 31 bits 	- containing the difference between the current time and the time in the header block expressed in FILETIME units.
	//			  This part of the child block is produced using one of two strategies, depending on the value of the first bit.
	//			  * If this bit is zero, ScCreateConversationIndex discards the high 15 bits and the low 18 bits.
	//			  * If this bit is one, the function discards the high 10 bits and the low 23 bits.
	// 4 bits 	- containing a random number generated by calling the Win32 function GetTickCount.
	// 4 bits 	- containing a sequence count that is taken from part of the random number.
	// -------------------------------------------------------------------------------------------------
	// ref: https://docs.microsoft.com/en-us/office/client-developer/outlook/mapi/tracking-conversations
	// -------------------------------------------------------------------------------------------------
	timeFlag := false
	deltaDuration := time.Duration(deltaTimeUxNs) * time.Nanosecond
	deltaYears := deltaDuration.Hours() / 24 / 365
	if deltaDuration.Seconds() <= 0.02 || deltaYears > 2 {
		timeFlag = false
	} else if deltaDuration.Seconds() <= 1 || deltaYears > 56 {
		timeFlag = true
	}

	// random num (last 1 Byte)
	rand.Seed(time.Now().UnixNano())
	randomNum := byte(rand.Intn(15))

	// sequence count
	sequenceCount := byte(0)

	return ChildBlock{
		timeFlag,
		deltaTimeUxNs,
		randomNum,
		sequenceCount,
	}
}
func ParseChildBlock(blockString string) (block ChildBlock, err error) {
	if len(blockString) < 0 || len(blockString) > 5 {
		return ChildBlock{}, errors.New("Block string is too short/long!")
	}

	bytes := []byte(blockString)

	//  1 bit - flag
	timeFlag := byte(bytes[0]&0x80) > 0

	// 31 bit - time - containing the difference between the current time and the time in the header block expressed in FILETIME units.This part of the child block is produced using one of two strategies, depending on the value of the first bit.
	//		If first bit is 1, the function discards the high 10 bits and the low 23 bits.
	//		If first bit is 0, ScCreateConversationIndex discards the high 15 bits and the low 18 bits.

	// add prefix + suffix
	dWord := binary.BigEndian.Uint32(bytes[:4])
	dWord = dWord & 0x7fffffff // remove first bit (timeFlag)
	qWord := uint64(dWord)
	if timeFlag {
		qWord = qWord << 23
	} else {
		qWord = qWord << 18
	}
	tsDiff := time.Duration(qWord*100) * time.Nanosecond

	//  4 bit - Random num
	rnd := bytes[4] & 0xF0 >> 4

	//  4 bit - sequence count
	seq := bytes[4] & 0x0F

	return ChildBlock{
		timeFlag,
		tsDiff.Nanoseconds(),
		rnd,
		seq,
	}, nil

}

// Bytes returns bits representing the Child block :
// 40 bits: 1 flag, 31 time diff, 4 random, 4 seq
func (block ChildBlock) Bytes() []byte {

	if block.TimeDifference == 0 {
		return nil
	}
	cbBytes := []byte{0, 0, 0, 0, 0}
	const FIRST_BIT_UP = uint64(0x80000000)

	// get first 4 bytes as follows:
	//  1  bit - time flag
	// 31 bits - containing the difference between the current time and the time in the header block expressed in FILETIME units.This part of the child block is produced using one of two strategies, depending on the value of the first bit.
	//		If first bit is 1, the function discards the high 10 bits and the low 23 bits.
	//		If first bit is 0, ScCreateConversationIndex discards the high 15 bits and the low 18 bits.
	tsDiff := uint64(block.TimeDifference / 100)

	// componse (first 4 bytes)
	if block.TimeFlag {
		tsDiff = tsDiff << 10
		tsDiff = tsDiff >> (10 + 23)
	} else {
		tsDiff = tsDiff << 15
		tsDiff = tsDiff >> (15 + 18)
	}
	// componse first bit
	if block.TimeFlag {
		tsDiff = tsDiff | FIRST_BIT_UP
	}
	binary.BigEndian.PutUint32(cbBytes, uint32(tsDiff))

	// get last 1 byte
	cbBytes[4] = (block.RandomNum << 4) | block.SequenceCount

	return cbBytes
}
func (block ChildBlock) String() string {
	return string(block.Bytes())
}

// Helping types -----------------------

// FILETIME
// represents the date and time for a file.
// It is a 64-bit value representing the number of 100-nanosecond intervals since January 1, 1601 (UTC)
// This is different from Unix time which is the number of nanoseconds elapsed since January 1, 1970, 00:00:00 (UTC)
// --------------------------
// 	Generic file time stamp :
// --------------------------
// 	31 30 29 28 27 26 25 24 23 22 21 20 19 18 17 16 	15 14 13 12 11 10  9  8  7  6  5  4  3  2  1  0
//  |<------ year ------>|<- month ->|<---- day --->|	|<--- hour --->|<---- minute --->|<- second/2 ->|
//
//    Offset   Length   Contents
// 	   0       7 bits   year     years since 1980
// 	   7       4 bits   month    [1..12]
//    11       5 bits   day      [1..31]
//    16       5 bits   hour     [0..23]
//    21       6 bits   minite   [0..59]
//    27       5 bits   second/2 [0..29]
// --------------------------
// ref: https://golang.org/src/syscall/types_windows.go
type Filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

// UnixNanoseconds returns Filetime in nanoseconds since Epoch (00:00:00 UTC, January 1, 1970).
func (ft *Filetime) UnixNanoseconds() int64 {

	// 100-nanosecond intervals since January 1, 1601
	nsec := int64(ft.HighDateTime)<<32 + int64(ft.LowDateTime)

	// change starting time to the Epoch (00:00:00 UTC, January 1, 1970)
	nsec -= 116444736000000000

	// convert into nanoseconds
	nsec *= 100
	return nsec
}
func UnixNanoToFiletime(nsec int64) (ft Filetime) {
	// convert into 100-nanosecond
	nsec /= 100

	// change starting time to January 1, 1601
	nsec += 116444736000000000

	// split into high / low
	ft.LowDateTime = uint32(nsec & 0xffffffff)
	ft.HighDateTime = uint32(nsec >> 32 & 0xffffffff)
	return ft
}

// Helping functions
func GetNormilizedSubject(subject string, level int) string {
	normalizedSubject := subject
	normalizedSubject = strings.TrimPrefix(normalizedSubject, "RE:")
	normalizedSubject = strings.TrimPrefix(normalizedSubject, "FW:")
	normalizedSubject = strings.TrimPrefix(normalizedSubject, " ")

	// do that again until we get rid of all prefixes
	if len(normalizedSubject) != len(subject) && level > 1 {
		normalizedSubject = GetNormilizedSubject(normalizedSubject, level-1)
	}
	return normalizedSubject
}
func ParseGuid(s string) uuid.UUID {
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
func TimeStampToUnix(timeStampTicks uint64) (unixNano uint64) {
	// 	timeStampTicks - a 64-bit value representing the number of 100-nanosecond intervals since January 1, 1601 (UTC)
	//	 	  UnixNano - the number of nanoseconds elapsed since January 1, 1970, 00:00:00 (UTC)
	return (timeStampTicks - 116444736000000000) * 100
}
func UnixToTimeStamp64(unixNanosecond int64) int64 {
	return unixNanosecond/100 + 116444736000000000
}
func UnixToTimeStamp(unixNanosecond uint64) uint64 {
	return unixNanosecond/100 + 116444736000000000
}

func IntToBytes(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}
func IntToHexU(num uint64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.LittleEndian, num)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}
func BytesToUInt64(s []byte) uint64 {
	var b [8]byte
	copy(b[8-len(s):], s)
	return uint64(binary.BigEndian.Uint64(b[:]))
}
func Hash(s string) []byte {
	h := sha1.New()
	h.Write([]byte(s))
	return h.Sum(nil)
}

func HexToBase64(bites []byte) string {
	return base64.StdEncoding.EncodeToString(bites)
}
