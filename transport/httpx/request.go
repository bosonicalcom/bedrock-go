package httpx

import (
	"fmt"
	"net/http"

	"github.com/bosonicalcom/bedrock-go/mimex"
	"github.com/bosonicalcom/bedrock-go/syserr"
	"github.com/bytedance/sonic"
)

// ParseRequestJSON decodes the JSON body of an HTTP request into a specified type.
func ParseRequestJSON[T any](r *http.Request) (T, error) {
	// verify server accepts json as body
	if r.Header.Get("Content-Type") != mimex.ApplicationJSON.String() {
		var zeroVal T
		return zeroVal, syserr.New(syserr.CodeInvalidArgument, fmt.Sprintf("unsupported content type %s", r.Header.Get("Content-Type")),
			syserr.WithDetails(
				syserr.ErrorInfo{
					Reason: string(syserr.CodeInvalidArgument),
					Domain: syserr.GlobalDomain,
					Metadata: map[string]string{
						"expected": mimex.ApplicationJSON.String(),
					},
				},
			),
		)
	}

	out := new(T)
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(out); err != nil {
		var zeroVal T
		return zeroVal, syserr.New(syserr.CodeInvalidArgument, "cannot decode request body",
			syserr.WithDetails(
				syserr.ErrorInfo{
					Reason: string(syserr.CodeInvalidArgument),
					Domain: syserr.GlobalDomain,
				},
				syserr.DebugInfo{
					Detail:       err.Error(),
					StackEntries: nil,
				},
			),
		)
	}
	return *out, nil
}
