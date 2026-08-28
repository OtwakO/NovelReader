//go:build opencc_native

package chineseconv

import (
	"context"
	"reflect"
	"testing"
)

func TestOfficialOpenCCJiebaGoldenOutputs(t *testing.T) {
	service, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	capability := service.Capability()
	if !capability.Available || capability.Engine != "BYVoid/OpenCC" || capability.Version != "1.4.2" || capability.Presets[ModeSimplified] != "tw2sp_jieba.json" || capability.Presets[ModeTraditionalTaiwan] != "s2twp_jieba.json" {
		t.Fatalf("unexpected capability: %+v", capability)
	}

	traditional, err := service.Convert(context.Background(), ModeTraditionalTaiwan, []string{"软件后台", "鼠标和硬盘", "数据库连接", "这里"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"軟體後臺", "滑鼠和硬碟", "資料庫連線", "這裡"}; !reflect.DeepEqual(traditional, want) {
		t.Fatalf("traditional=%q want=%q", traditional, want)
	}

	simplified, err := service.Convert(context.Background(), ModeSimplified, []string{"軟體後臺", "滑鼠和硬碟", "資料庫連線", "這裡"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"软件后台", "鼠标和硬盘", "数据库连接", "这里"}; !reflect.DeepEqual(simplified, want) {
		t.Fatalf("simplified=%q want=%q", simplified, want)
	}
}
