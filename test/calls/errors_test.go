package calls_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/green-api/whatsapp-chatbot-golang/calls"
)

func TestProviderErrorContract(t *testing.T) {
	cause := errors.New("token=secret")
	provider := &calls.ProviderError{Provider: "custom", Code: "dial_failed", Detail: "private diagnostic", Cause: cause}
	err := errors.Join(errors.New("execution failed"), provider)
	var got *calls.ProviderError
	if !errors.As(err, &got) || got != provider || !errors.Is(err, cause) {
		t.Fatal("provider identity or original cause lost")
	}
	if provider.Error() != "custom [dial_failed]" || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret") {
		t.Fatal("provider diagnostics exposed")
	}
	withoutCause := &calls.ProviderError{Provider: "custom", Code: "rejected"}
	if withoutCause.Unwrap() != nil {
		t.Fatal("provider supplied a synthetic cause")
	}
}
