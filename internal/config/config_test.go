// 文件说明：验证当前配置可加载、默认值有效以及配置内容指纹稳定。
package config

import (
	"path/filepath"
	"testing"
)

func TestCurrentConfigsLoad(t *testing.T) {
	paths, err := filepath.Glob("../../configs/local/*.json")
	if err != nil || len(paths) == 0 {
		t.Fatal("missing configurations", err)
	}
	for _, path := range paths {
		app, hash, err := Load(path)
		if err != nil {
			t.Fatal(path, err)
		}
		_, again, err := Load(path)
		if err != nil || hash != again || app.Limit.MaxBodyBytes <= 0 {
			t.Fatal("invalid defaults or fingerprint")
		}
	}
	bad := App{}
	if bad.Validate() == nil {
		t.Fatal("empty config accepted")
	}
}
