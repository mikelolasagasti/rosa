package log

import (
	"regexp"
)

const (
	RedactValue = "*************"
)
const (
	info  = "INFO"
	debug = "DEBUG"
	fatal = "FATAL"
	warn  = "WARN"
	err   = "ERROR"
)

var RedactKeyList = []*regexp.Regexp{
	regexp.MustCompile(`(\\?"password\\?":\\?")([^"]*)(\\?")`),
	regexp.MustCompile(`(\\?"additional_trust_bundle\\?":\\?")([^"]*)(\\?")`),
	regexp.MustCompile(`(-----BEGIN CERTIFICATE-----\n)([^-----]*)(-----END CERTIFICATE-----)`),
	regexp.MustCompile(`(--password\s+)([^\n\t\\\s]+)([\n\\\s]*)`),
	regexp.MustCompile(`(--client-id\s+)([^\n\t\\\s]+)([\n\\\s]*)`),
	regexp.MustCompile(`(--bind-password\s+)([^\n\t\\\s]+)([\n\\\s]*)`),
	regexp.MustCompile(`(--client-secret\s+)([^\n\t\\\s]+)([\n\\\s]*)`),
	regexp.MustCompile(`(--users\s+[a-zA-Z0-9-]+:)([^\s\n\\]+)([\s\S]+)`),
	regexp.MustCompile(`(--cluster-admin-password\s+)([^\n\t\\\s]+)([\n\\\s]*)`),
	regexp.MustCompile(`(--billing-account\s+)([^\n\t\\\s]+)([\n\\\s]*)`),
	regexp.MustCompile(`(arn:aws:[a-z]+:[a-z0-9-]*:)([0-9]{12})(:)`),
	regexp.MustCompile(`(AWS Account:\s*)([0-9]{12})([\n\\n].)`),
	regexp.MustCompile(`(AWS Billing Account:\s*)([0-9]{12})([\n\\n].)`),
}
