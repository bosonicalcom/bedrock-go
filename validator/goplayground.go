package validator

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"

	"github.com/bosonicalcom/bedrock-go/syserr"
)

var _tagNames = []string{"json", "env", "yaml", "toml", "xml"}

// GoPlaygroundValidate creates a new validator instance with no extra options.
//
// It is a singleton instance, ensuring that the same configuration is used across the application.
func GoPlaygroundValidate() *validator.Validate {
	return sync.OnceValue(func() *validator.Validate {
		v := validator.New()
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			for _, tag := range _tagNames {
				name := strings.SplitN(fld.Tag.Get(tag), ",", 2)[0]
				if name != "" && name != "-" {
					return name
				}
			}
			// skip if tag key says it should be ignored
			return ""
		})
		return v
	})()
}

// GoPlaygroundValidator is a struct that implements the Validator interface, acting like an adapter to the Go
// Playground Validator, translating validation errors into the syserr system errors.
type GoPlaygroundValidator struct{}

var _ Validator = (*GoPlaygroundValidator)(nil)

// Validate implements [Validator] by running the Go Playground validator against v
// and translating any [validator.ValidationErrors] into a [syserr.Error] with
// [syserr.CodeInvalidArgument] and a [syserr.BadRequest] detail listing each violation.
func (g GoPlaygroundValidator) Validate(ctx context.Context, v any) error {
	err := GoPlaygroundValidate().StructCtx(ctx, v)
	if err == nil {
		return nil
	}

	// unwrap validations errors and convert them into syserrs.

	validationErrs, ok := errors.AsType[validator.ValidationErrors](err)
	if !ok {
		return err
	}

	violations := make([]syserr.FieldViolation, 0, len(validationErrs))
	for _, validationErr := range validationErrs {
		// localeMsgKey := fmt.Sprintf("global.errors.field_validations.%s", validationErr.Tag())
		fieldNameSplit := strings.Split(validationErr.Namespace(), ".")
		var fieldName string
		if len(fieldNameSplit) <= 1 {
			fieldName = validationErr.Namespace()
		} else {
			fieldName = strings.Join(fieldNameSplit[1:], ".") // remove struct name
		}
		validationFieldErr := syserr.FieldViolation{
			Field:       fieldName,
			Description: validationErr.Error(),
			Reason:      newReason(validationErr),
			// LocalizedMessage: i18n.NewLocalizedMessageError(ctx, localeMsgKey,
			// 	i18n.WithMessageFallback(validationErr.Error()),
			// 	i18n.WithMessageAttribute("Field", fieldName),
			// 	i18n.WithMessageAttribute("Options", validationErr.Param()),
			// 	i18n.WithMessageAttribute("Value", validationErr.Value()),
			// ),
		}
		violations = append(violations, validationFieldErr)
	}

	return syserr.New(syserr.CodeInvalidArgument, "validation failed",
		syserr.WithDetails(
			syserr.ErrorInfo{
				Reason: syserr.ReasonValidationFailed,
				Domain: syserr.GlobalDomain,
			},
			syserr.BadRequest{
				Violations: violations,
			},
			// i18n.NewLocalizedMessageError(ctx, "global.errors.field_validations.message",
			// 	i18n.WithMessageFallback(validationErrs.Error()),
			// ),
		),
	)
}

// - util

var reasonByTag = map[string]string{
	// presence
	"required":             syserr.ReasonRequiredValue,
	"required_if":          syserr.ReasonRequiredValue,
	"required_unless":      syserr.ReasonRequiredValue,
	"required_with":        syserr.ReasonRequiredValue,
	"required_with_all":    syserr.ReasonRequiredValue,
	"required_without":     syserr.ReasonRequiredValue,
	"required_without_all": syserr.ReasonRequiredValue,

	// exclusions
	"excluded_if":          syserr.ReasonForbiddenValue,
	"excluded_unless":      syserr.ReasonForbiddenValue,
	"excluded_with":        syserr.ReasonForbiddenValue,
	"excluded_with_all":    syserr.ReasonForbiddenValue,
	"excluded_without":     syserr.ReasonForbiddenValue,
	"excluded_without_all": syserr.ReasonForbiddenValue,

	// enumerations / exact allowed set
	"oneof": syserr.ReasonNotOneOfValue,

	// equality / mismatch
	"eq":             syserr.ReasonValueMismatch,
	"eq_ignore_case": syserr.ReasonValueMismatch,

	// forbidden equality / forbidden content
	"ne":             syserr.ReasonForbiddenValue,
	"ne_ignore_case": syserr.ReasonForbiddenValue,
	"excludes":       syserr.ReasonForbiddenValue,
	"excludesall":    syserr.ReasonForbiddenValue,
	"excludesrune":   syserr.ReasonForbiddenValue,
	"fieldexcludes":  syserr.ReasonForbiddenValue,
	"startsnotwith":  syserr.ReasonForbiddenValue,
	"endsnotwith":    syserr.ReasonForbiddenValue,

	// cross-field / relational constraints
	"eqfield":       syserr.ReasonInvalidRelation,
	"eqcsfield":     syserr.ReasonInvalidRelation,
	"nefield":       syserr.ReasonInvalidRelation,
	"necsfield":     syserr.ReasonInvalidRelation,
	"gtfield":       syserr.ReasonInvalidRelation,
	"gtefield":      syserr.ReasonInvalidRelation,
	"ltfield":       syserr.ReasonInvalidRelation,
	"ltefield":      syserr.ReasonInvalidRelation,
	"gtcsfield":     syserr.ReasonInvalidRelation,
	"gtecsfield":    syserr.ReasonInvalidRelation,
	"ltcsfield":     syserr.ReasonInvalidRelation,
	"ltecsfield":    syserr.ReasonInvalidRelation,
	"fieldcontains": syserr.ReasonInvalidRelation,

	// uniqueness
	"unique": syserr.ReasonDuplicateValue,

	// format-ish validators
	"alpha":                         syserr.ReasonInvalidFormat,
	"alphaspace":                    syserr.ReasonInvalidFormat,
	"alphanum":                      syserr.ReasonInvalidFormat,
	"alphanumspace":                 syserr.ReasonInvalidFormat,
	"alphaunicode":                  syserr.ReasonInvalidFormat,
	"alphanumunicode":               syserr.ReasonInvalidFormat,
	"ascii":                         syserr.ReasonInvalidFormat,
	"multibyte":                     syserr.ReasonInvalidFormat,
	"number":                        syserr.ReasonInvalidFormat,
	"numeric":                       syserr.ReasonInvalidFormat,
	"lowercase":                     syserr.ReasonInvalidFormat,
	"uppercase":                     syserr.ReasonInvalidFormat,
	"boolean":                       syserr.ReasonInvalidFormat,
	"hexadecimal":                   syserr.ReasonInvalidFormat,
	"hexcolor":                      syserr.ReasonInvalidFormat,
	"rgb":                           syserr.ReasonInvalidFormat,
	"rgba":                          syserr.ReasonInvalidFormat,
	"hsl":                           syserr.ReasonInvalidFormat,
	"hsla":                          syserr.ReasonInvalidFormat,
	"base64":                        syserr.ReasonInvalidFormat,
	"base64url":                     syserr.ReasonInvalidFormat,
	"base64rawurl":                  syserr.ReasonInvalidFormat,
	"json":                          syserr.ReasonInvalidFormat,
	"jwt":                           syserr.ReasonInvalidFormat,
	"email":                         syserr.ReasonInvalidFormat,
	"e164":                          syserr.ReasonInvalidFormat,
	"url":                           syserr.ReasonInvalidFormat,
	"http_url":                      syserr.ReasonInvalidFormat,
	"https_url":                     syserr.ReasonInvalidFormat,
	"uri":                           syserr.ReasonInvalidFormat,
	"url_encoded":                   syserr.ReasonInvalidFormat,
	"urn_rfc2141":                   syserr.ReasonInvalidFormat,
	"hostname":                      syserr.ReasonInvalidFormat,
	"hostname_rfc1123":              syserr.ReasonInvalidFormat,
	"hostname_port":                 syserr.ReasonInvalidFormat,
	"fqdn":                          syserr.ReasonInvalidFormat,
	"ip":                            syserr.ReasonInvalidFormat,
	"ip_addr":                       syserr.ReasonInvalidFormat,
	"ip4_addr":                      syserr.ReasonInvalidFormat,
	"ip6_addr":                      syserr.ReasonInvalidFormat,
	"ipv4":                          syserr.ReasonInvalidFormat,
	"ipv6":                          syserr.ReasonInvalidFormat,
	"cidr":                          syserr.ReasonInvalidFormat,
	"cidrv4":                        syserr.ReasonInvalidFormat,
	"cidrv6":                        syserr.ReasonInvalidFormat,
	"tcp_addr":                      syserr.ReasonInvalidFormat,
	"tcp4_addr":                     syserr.ReasonInvalidFormat,
	"tcp6_addr":                     syserr.ReasonInvalidFormat,
	"udp_addr":                      syserr.ReasonInvalidFormat,
	"udp4_addr":                     syserr.ReasonInvalidFormat,
	"udp6_addr":                     syserr.ReasonInvalidFormat,
	"mac":                           syserr.ReasonInvalidFormat,
	"uuid":                          syserr.ReasonInvalidFormat,
	"uuid3":                         syserr.ReasonInvalidFormat,
	"uuid4":                         syserr.ReasonInvalidFormat,
	"uuid5":                         syserr.ReasonInvalidFormat,
	"uuid_rfc4122":                  syserr.ReasonInvalidFormat,
	"uuid3_rfc4122":                 syserr.ReasonInvalidFormat,
	"uuid4_rfc4122":                 syserr.ReasonInvalidFormat,
	"uuid5_rfc4122":                 syserr.ReasonInvalidFormat,
	"ulid":                          syserr.ReasonInvalidFormat,
	"semver":                        syserr.ReasonInvalidFormat,
	"datetime":                      syserr.ReasonInvalidFormat,
	"timezone":                      syserr.ReasonInvalidFormat,
	"latitude":                      syserr.ReasonInvalidFormat,
	"longitude":                     syserr.ReasonInvalidFormat,
	"isbn":                          syserr.ReasonInvalidFormat,
	"isbn10":                        syserr.ReasonInvalidFormat,
	"isbn13":                        syserr.ReasonInvalidFormat,
	"issn":                          syserr.ReasonInvalidFormat,
	"credit_card":                   syserr.ReasonInvalidFormat,
	"postcode_iso3166_alpha2":       syserr.ReasonInvalidFormat,
	"postcode_iso3166_alpha2_field": syserr.ReasonInvalidFormat,
	"iso3166_1_alpha2":              syserr.ReasonInvalidFormat,
	"iso3166_1_alpha3":              syserr.ReasonInvalidFormat,
	"iso3166_1_alpha_numeric":       syserr.ReasonInvalidFormat,
	"iso3166_2":                     syserr.ReasonInvalidFormat,
	"iso4217":                       syserr.ReasonInvalidFormat,
	"bcp47_language_tag":            syserr.ReasonInvalidFormat,
	"cron":                          syserr.ReasonInvalidFormat,
	"mongodb":                       syserr.ReasonInvalidFormat,
	"mongodb_connection_string":     syserr.ReasonInvalidFormat,
	"spicedb":                       syserr.ReasonInvalidFormat,
	"cve":                           syserr.ReasonInvalidFormat,
}

func newReason(fe validator.FieldError) string {
	// Prefer stable alias names for aliases you intentionally expose.
	switch strings.ToLower(fe.Tag()) {
	case "iscolor", "country_code":
		return syserr.ReasonInvalidFormat
	}

	tag := strings.ToLower(fe.ActualTag())

	if reason, ok := reasonByTag[tag]; ok {
		return reason
	}

	switch tag {
	case "min":
		return minReason(fe.Kind())
	case "max":
		return maxReason(fe.Kind())
	case "len":
		return lenReason(fe.Kind())
	case "gt", "gte":
		return gteReason(fe.Kind())
	case "lt", "lte":
		return lteReason(fe.Kind())
	case "contains", "containsany", "containsrune", "startswith", "endswith":
		return syserr.ReasonValueMismatch
	default:
		slog.Warn("unmapped validator tag",
			"tag", fe.Tag(),
			"actual_tag", fe.ActualTag(),
			"field", fe.Namespace(),
			"param", fe.Param(),
		)
		return syserr.ReasonInvalidInput
	}
}

func minReason(kind reflect.Kind) string {
	switch kind {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return syserr.ReasonInvalidLength
	default:
		return syserr.ReasonValueTooSmall
	}
}

func maxReason(kind reflect.Kind) string {
	switch kind {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return syserr.ReasonInvalidLength
	default:
		return syserr.ReasonValueTooLarge
	}
}

func lenReason(kind reflect.Kind) string {
	switch kind {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return syserr.ReasonInvalidLength
	default:
		return syserr.ReasonValueOutOfRange
	}
}

func gteReason(kind reflect.Kind) string {
	switch kind {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return syserr.ReasonInvalidLength
	default:
		return syserr.ReasonValueTooSmall
	}
}

func lteReason(kind reflect.Kind) string {
	switch kind {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return syserr.ReasonInvalidLength
	default:
		return syserr.ReasonValueTooLarge
	}
}
