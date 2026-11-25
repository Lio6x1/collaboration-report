package contractscheck

import (
	"context"
	"github.com/getkin/kin-openapi/openapi3"
	"testing"
)

func TestProductOpenAPI(t *testing.T) {
	// 在正式项目副本上校验所有引用、参数和响应，不能只检查文件存在。
	doc, err := openapi3.NewLoader().LoadFromFile("../../contracts/http/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
}
