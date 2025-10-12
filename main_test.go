package main

import (
	"fmt"
	"os/exec"
	"testing"
	"time"
)

func addEventToCalendar(title, startTime, endTime string) error {
	// 解析开始时间和结束时间
	start, err := time.Parse("2006-01-02 15:04:05", startTime)
	if err != nil {
		fmt.Printf("解析开始时间失败: %v", err)
		return err
	}
	end, err := time.Parse("2006-01-02 15:04:05:", endTime)
	if err != nil {
		fmt.Printf("解析结束时间失败: %v", err)
		return err
	}

	// 格式化时间为AppleScript所需的格式
	startFormatted := start.Format("yyyy-MM-dd HH:mm:ss")
	endFormatted := end.Format("yyyy-MM-dd HH:mm:ss")

	// 构建osascript命令
	script := fmt.Sprintf(`tell application "Calendar"
        set newEvent to make new event at end of events with properties {summary:"%s", start date:date "%s", end date:date "%s"}
    end tell`, title, startFormatted, endFormatted)

	// 执行osascript命令
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("执行命令失败: %v输出: %s", err, output)
		return err
	}
	return nil
}

func Test_generateICS(t *testing.T) {
	title := "test command"
	startTime := "2025-02-27 11:00:00" // 事件开始时间
	endTime := "2025-02-27 12:00:00"   // 事件结束时间

	//Wednesday, May 29, 2024 at 9:15:00 AM

	err := addEventToCalendar(title, startTime, endTime)
	if err != nil {
		fmt.Printf("添加事件失败: %v", err)
		return
	}

	fmt.Println("事件已成功添加到 Apple Calendar！")
}

func Test_update(t *testing.T) {
	//update("")
}

func Test_syncNotion(t *testing.T) {
	// 从配置文件读取敏感信息
	conf, err := InitConfig()
	if err != nil {
		t.Fatalf("Failed to init config: %v", err)
	}

	data, err := getNotionDatabaseData(conf.Key, conf.Databases)
	if err != nil {
		t.Error(err)
		return
	}
	// 打印获取到的数据库数据
	t.Logf("获取到 %d 条记录", len(data.Results))
}
