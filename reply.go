// Copyright (c) 2011 Ross Light.

package ftp

import (
	"strconv"
	"strings"
)

// Code is an FTP reply code.
type Code int

// FTP reply codes defined in RFC 959.
const (
	CodeRestartMarker    Code = 110
	CodeServiceReadySoon Code = 120
	CodeStartingTransfer Code = 125
	CodeFileStatusOkay   Code = 150

	CodeOkay            Code = 200
	CodeSuperfluous     Code = 202
	CodeSystemStatus    Code = 211
	CodeDirectoryStatus Code = 212
	CodeFileStatus      Code = 213
	CodeHelpMessage     Code = 214
	CodeSystemType      Code = 215
	CodeServiceReady    Code = 220
	CodeServiceClosing  Code = 221
	CodeNoTransfer      Code = 225
	CodeClosingData     Code = 226
	CodePassive         Code = 227
	CodeExtendedPassive Code = 229
	CodeLoggedIn        Code = 230
	CodeActionOkay      Code = 250
	CodeCreated         Code = 257

	CodeNeedPassword       Code = 331
	CodeNeedAccount        Code = 332
	CodePendingInformation Code = 350

	CodeServiceNotAvailable Code = 421
	CodeCantOpenData        Code = 425
	CodeTransferAborted     Code = 426
	CodeActionNotTaken      Code = 450
	CodeLocalError          Code = 451
	CodeInsufficientStorage Code = 452

	CodeUnrecognizedCommand     Code = 500
	CodeParameterSyntaxError    Code = 501
	CodeNotImplemented          Code = 502
	CodeBadSequence             Code = 503
	CodeParameterNotImplemented Code = 504
	CodeNotLoggedIn             Code = 530
	CodeNoAccount               Code = 532
	CodeFileUnavailable         Code = 550
	CodePageTypeUnknown         Code = 551
	CodeExceededQuota           Code = 552
	CodeFileNameNotAllowed      Code = 553
)

// Preliminary returns whether the code indicates a preliminary positive reply.
func (code Code) Preliminary() bool {
	return code/100 == 1
}

// Positive returns whether the code is positive.
func (code Code) Positive() bool {
	firstDigit := code / 100
	return firstDigit == 1 || firstDigit == 2 || firstDigit == 3
}

// Complete returns whether this code indicates a complete reply.  A complete reply code is not
// necessarily positive.
func (code Code) Complete() bool {
	firstDigit := code / 100
	return firstDigit == 2 || firstDigit == 4 || firstDigit == 5
}

// PositiveComplete returns whether this code is a positive completion.
func (code Code) PositiveComplete() bool {
	return code/100 == 2
}

// Temporary returns whether this code indicates a temporary error.
func (code Code) Temporary() bool {
	return code/100 == 4
}

func (code Code) String() string {
	return strconv.Itoa(int(code))
}

// Reply is a response from a server.  This may also be used as an error.
type Reply struct {
	Code
	Msg string
}

func (r Reply) String() string {
	lines := strings.Split(r.Msg, "\n")
	if len(lines) > 1 {
		lines[0] = r.Code.String() + "-" + lines[0]
		lines[len(lines)-1] = r.Code.String() + " " + lines[len(lines)-1]
		return strings.Join(lines, "\r\n")
	}
	return r.Code.String() + " " + r.Msg
}

func (r Reply) Error() string {
	return r.String()
}
