package platform

import "testing"

func TestProtectedAccountAndDocumentOutputsDoNotUseDetachedTimeoutBuffer(t *testing.T) {
	document, err := RESTConfig(Config{Role: "document"})
	if err != nil {
		t.Fatal(err)
	}
	if document.Middlewares.Timeout {
		t.Fatal("document output has detached response buffer")
	}
	account, err := RESTConfig(Config{Role: "account"})
	if err != nil {
		t.Fatal(err)
	}
	if account.Middlewares.Timeout {
		t.Fatal("account protected outputs must settle synchronously before acknowledgement")
	}
}
