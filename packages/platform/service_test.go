package platform

import (
	"testing"
)

func TestDefaultFrameworkDebugServerRequiresExplicitDisable(t *testing.T) {
	rc, err := RESTConfig(Config{Role: "document", Host: "127.0.0.1", HTTPPort: 20003})
	if err != nil {
		t.Fatal(err)
	}
	// 项目配置必须关闭未归属监督器的额外HTTP/pprof端口。
	if rc.DevServer.Enabled {
		t.Fatal("default framework would expose an extra debug listener")
	}
}
