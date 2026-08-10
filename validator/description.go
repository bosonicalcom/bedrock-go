package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// newDescription builds a client-safe description of a field violation.
//
// It replaces [validator.FieldError.Error], whose output ("Key: 'CreateUserRequest.country'
// Error:Field validation for 'country' failed on the 'iso3166_1_alpha2' tag") names the Go
// struct and the validator tag and is only meaningful to someone reading the source.
// Descriptions here are phrased as predicates of the field, which is already identified by
// [syserr.FieldViolation.Field], e.g. "must be a valid email address".
func newDescription(fe validator.FieldError) string {
	// mirror newReason: prefer the alias name for aliases we intentionally expose.
	switch strings.ToLower(fe.Tag()) {
	case "iscolor":
		return "must be a valid color"
	case "country_code":
		return "must be a valid country code"
	}

	tag := strings.ToLower(fe.ActualTag())
	param := fe.Param()

	switch tag {
	// presence
	case "required", "required_if", "required_unless",
		"required_with", "required_with_all", "required_without", "required_without_all":
		return "is required"

	// exclusions
	case "excluded_if", "excluded_unless",
		"excluded_with", "excluded_with_all", "excluded_without", "excluded_without_all":
		return "must not be provided"

	// enumerations
	case "oneof":
		return "must be one of: " + strings.Join(strings.Fields(param), ", ")

	// equality
	case "eq", "eq_ignore_case":
		return fmt.Sprintf("must equal %q", param)
	case "ne", "ne_ignore_case":
		return fmt.Sprintf("must not equal %q", param)

	// content
	case "contains", "containsany", "containsrune":
		return fmt.Sprintf("must contain %q", param)
	case "excludes", "excludesall", "excludesrune":
		return fmt.Sprintf("must not contain %q", param)
	case "startswith":
		return fmt.Sprintf("must start with %q", param)
	case "endswith":
		return fmt.Sprintf("must end with %q", param)
	case "startsnotwith":
		return fmt.Sprintf("must not start with %q", param)
	case "endsnotwith":
		return fmt.Sprintf("must not end with %q", param)

	// uniqueness
	case "unique":
		return "must not contain duplicate values"

	// bounds
	case "min", "gte":
		return boundDescription(fe.Kind(), "at least", param)
	case "max", "lte":
		return boundDescription(fe.Kind(), "at most", param)
	case "gt":
		return boundDescription(fe.Kind(), "more than", param)
	case "lt":
		return boundDescription(fe.Kind(), "less than", param)
	case "len":
		return lengthDescription(fe.Kind(), param)

	// parameterized formats
	case "datetime":
		return fmt.Sprintf("must be a valid date/time in the format %q", param)
	case "postcode_iso3166_alpha2":
		return fmt.Sprintf("must be a valid postal code for country %q", param)
	case "postcode_iso3166_alpha2_field":
		return "must be a valid postal code for the given country"
	}

	// cross-field relations. The param is the peer's Go field name, which is the only
	// identifier go-playground exposes here; there is no way back to its json tag name.
	if relation, ok := relationByTag[tag]; ok {
		return fmt.Sprintf("%s the %s field", relation, param)
	}

	if description, ok := descriptionByTag[tag]; ok {
		return description
	}

	return "is invalid"
}

// boundDescription renders a minimum/maximum constraint in the terms that suit the kind
// being validated: characters for strings, items for collections, magnitude otherwise.
func boundDescription(kind reflect.Kind, bound, param string) string {
	switch kind {
	case reflect.String:
		return fmt.Sprintf("must be %s %s long", bound, quantity(param, "character"))
	case reflect.Array, reflect.Slice, reflect.Map:
		return fmt.Sprintf("must contain %s %s", bound, quantity(param, "item"))
	default:
		return fmt.Sprintf("must be %s %s", bound, param)
	}
}

// lengthDescription renders an exact-length constraint.
func lengthDescription(kind reflect.Kind, param string) string {
	switch kind {
	case reflect.String:
		return fmt.Sprintf("must be exactly %s long", quantity(param, "character"))
	case reflect.Array, reflect.Slice, reflect.Map:
		return fmt.Sprintf("must contain exactly %s", quantity(param, "item"))
	default:
		return fmt.Sprintf("must equal %s", param)
	}
}

// quantity pairs a tag parameter with its unit, pluralizing everything but exactly one.
func quantity(param, unit string) string {
	if param == "1" {
		return "1 " + unit
	}
	return param + " " + unit + "s"
}

// relationByTag maps a cross-field tag to the comparison it expresses.
var relationByTag = map[string]string{
	"eqfield":       "must equal",
	"eqcsfield":     "must equal",
	"nefield":       "must not equal",
	"necsfield":     "must not equal",
	"gtfield":       "must be greater than",
	"gtcsfield":     "must be greater than",
	"gtefield":      "must be greater than or equal to",
	"gtecsfield":    "must be greater than or equal to",
	"ltfield":       "must be less than",
	"ltcsfield":     "must be less than",
	"ltefield":      "must be less than or equal to",
	"ltecsfield":    "must be less than or equal to",
	"fieldcontains": "must contain",
	"fieldexcludes": "must not contain",
}

// descriptionByTag maps a format-style validator tag to its client-facing description.
// Tags whose description depends on the tag parameter are handled in [newDescription].
var descriptionByTag = map[string]string{
	// character classes
	"alpha":           "must contain only letters",
	"alphaspace":      "must contain only letters and spaces",
	"alphanum":        "must contain only letters and digits",
	"alphanumspace":   "must contain only letters, digits and spaces",
	"alphaunicode":    "must contain only unicode letters",
	"alphanumunicode": "must contain only unicode letters and digits",
	"ascii":           "must contain only ASCII characters",
	"multibyte":       "must contain multibyte characters",
	"lowercase":       "must be lowercase",
	"uppercase":       "must be uppercase",

	// scalars
	"number":      "must be a number",
	"numeric":     "must be a numeric value",
	"boolean":     "must be a boolean value",
	"hexadecimal": "must be a hexadecimal value",

	// colors
	"hexcolor": "must be a valid hex color",
	"rgb":      "must be a valid RGB color",
	"rgba":     "must be a valid RGBA color",
	"hsl":      "must be a valid HSL color",
	"hsla":     "must be a valid HSLA color",

	// encodings
	"base64":       "must be a valid base64 string",
	"base64url":    "must be a valid base64url string",
	"base64rawurl": "must be a valid unpadded base64url string",
	"json":         "must be valid JSON",
	"jwt":          "must be a valid JWT",
	"url_encoded":  "must be URL-encoded",

	// contact
	"email": "must be a valid email address",
	"e164":  "must be a valid E.164 phone number",

	// locators
	"url":              "must be a valid URL",
	"http_url":         "must be a valid HTTP URL",
	"https_url":        "must be a valid HTTPS URL",
	"uri":              "must be a valid URI",
	"urn_rfc2141":      "must be a valid URN",
	"hostname":         "must be a valid hostname",
	"hostname_rfc1123": "must be a valid RFC 1123 hostname",
	"hostname_port":    "must be a valid host:port pair",
	"fqdn":             "must be a fully qualified domain name",

	// network
	"ip":        "must be a valid IP address",
	"ip_addr":   "must be a resolvable IP address",
	"ip4_addr":  "must be a resolvable IPv4 address",
	"ip6_addr":  "must be a resolvable IPv6 address",
	"ipv4":      "must be a valid IPv4 address",
	"ipv6":      "must be a valid IPv6 address",
	"cidr":      "must be a valid CIDR block",
	"cidrv4":    "must be a valid IPv4 CIDR block",
	"cidrv6":    "must be a valid IPv6 CIDR block",
	"tcp_addr":  "must be a valid TCP address",
	"tcp4_addr": "must be a valid TCP IPv4 address",
	"tcp6_addr": "must be a valid TCP IPv6 address",
	"udp_addr":  "must be a valid UDP address",
	"udp4_addr": "must be a valid UDP IPv4 address",
	"udp6_addr": "must be a valid UDP IPv6 address",
	"mac":       "must be a valid MAC address",

	// identifiers
	"uuid":          "must be a valid UUID",
	"uuid3":         "must be a valid version 3 UUID",
	"uuid4":         "must be a valid version 4 UUID",
	"uuid5":         "must be a valid version 5 UUID",
	"uuid_rfc4122":  "must be a valid RFC 4122 UUID",
	"uuid3_rfc4122": "must be a valid RFC 4122 version 3 UUID",
	"uuid4_rfc4122": "must be a valid RFC 4122 version 4 UUID",
	"uuid5_rfc4122": "must be a valid RFC 4122 version 5 UUID",
	"ulid":          "must be a valid ULID",
	"semver":        "must be a valid semantic version",
	"mongodb":       "must be a valid MongoDB ObjectID",
	"spicedb":       "must be a valid SpiceDB identifier",
	"cve":           "must be a valid CVE identifier",

	// standards
	"iso3166_1_alpha2":        "must be a valid ISO 3166-1 alpha-2 country code",
	"iso3166_1_alpha3":        "must be a valid ISO 3166-1 alpha-3 country code",
	"iso3166_1_alpha_numeric": "must be a valid ISO 3166-1 numeric country code",
	"iso3166_2":               "must be a valid ISO 3166-2 subdivision code",
	"iso4217":                 "must be a valid ISO 4217 currency code",
	"bcp47_language_tag":      "must be a valid BCP 47 language tag",
	"isbn":                    "must be a valid ISBN",
	"isbn10":                  "must be a valid ISBN-10",
	"isbn13":                  "must be a valid ISBN-13",
	"issn":                    "must be a valid ISSN",
	"credit_card":             "must be a valid credit card number",

	// misc
	"timezone":                  "must be a valid time zone",
	"latitude":                  "must be a valid latitude",
	"longitude":                 "must be a valid longitude",
	"cron":                      "must be a valid cron expression",
	"mongodb_connection_string": "must be a valid MongoDB connection string",
}
