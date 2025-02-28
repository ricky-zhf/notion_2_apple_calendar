package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	ical "github.com/arran4/golang-ical"
)

type Conf struct {
	Key       string `json:"key"`
	Path      string `json:"path"`
	Databases string `json:"databases"`
}

// NotionDatabase 定义 Notion 数据库响应结构
type NotionDatabase struct {
	Results []struct {
		Properties struct {
			Name struct {
				Title []struct {
					Text struct {
						Content string `json:"content"`
					} `json:"text"`
				} `json:"title"`
			} `json:"Name"`
			Date struct {
				Date struct {
					Start string `json:"start"`
					End   string `json:"end"`
				} `json:"date"`
			} `json:"Time"`
		} `json:"properties"`
	} `json:"results"`
}

// 获取 Notion 数据库数据
func getNotionDatabaseData(token, databaseID string) (*NotionDatabase, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", databaseID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var database NotionDatabase
	err = json.Unmarshal(body, &database)
	if err != nil {
		return nil, err
	}

	return &database, nil
}

var layout = "2006-01-02T15:04:05.000Z07:00"

// 生成 .ics 文件内容
func generateICS(c Conf, database *NotionDatabase) ([]byte, error) {
	cal := ical.NewCalendar()
	cal.SetMethod(ical.MethodPublish)
	cal.SetProductId("-//Example//Example Calendar//EN")

	for _, result := range database.Results {
		event := cal.AddEvent(fmt.Sprintf("%d", time.Now().UnixNano()))
		if len(result.Properties.Name.Title) == 0 {
			continue
		}
		event.SetSummary(result.Properties.Name.Title[0].Text.Content)
		if len(result.Properties.Date.Date.Start) > 0 && len(result.Properties.Date.Date.End) > 0 {
			start, err := time.Parse(layout, result.Properties.Date.Date.Start)
			if err != nil {
				return nil, err
			}
			event.SetStartAt(start)
			end, err := time.Parse(layout, result.Properties.Date.Date.End)
			if err != nil {
				return nil, err
			}
			event.SetEndAt(end)
		}
	}
	var buf bytes.Buffer
	err := cal.SerializeTo(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func main() {
	// 获取可执行文件的绝对路径
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("获取可执行文件路径失败: %v\n", err)
		return
	}

	// 获取可执行文件所在目录
	exeDir := filepath.Dir(exePath)

	// 拼接配置文件的绝对路径
	confPath := filepath.Join(exeDir, "conf.json")

	confData, err := os.ReadFile(confPath)
	if err != nil {
		fmt.Printf("读取配置文件失败: %v\n", err)
		return
	}
	var conf Conf
	err = json.Unmarshal(confData, &conf)
	if err != nil {
		fmt.Printf("解析配置文件失败: %v\n", err)
		return
	}

	syncNotion(conf)
	fmt.Println("get notion end...")

	go runServer()

	fmt.Println("start http end...")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			syncNotion(conf)
		}
	}
}

func syncNotion(c Conf) {
	defer func() {
		_ = recover()
	}()
	// 替换为你的 Notion API 密钥和数据库 ID
	notionToken := c.Key
	databaseID := c.Databases
	//https://www.notion.so/rickyzhf/16eea8b06b7f8083b50bcd90ecb5397f?v=16eea8b06b7f81009b03000c49e7c0b7&pvs=4

	// 获取 Notion 数据库数据
	database, err := getNotionDatabaseData(notionToken, databaseID)
	if err != nil {
		//log.Fatalf("Failed to get Notion database data: %v", err)
		return
	}

	// 生成 .ics 文件内容
	icsData, err := generateICS(c, database)
	if err != nil {
		//log.Fatalf("Failed to generate ICS data: %v", err)
		return
	}

	// 保存 .ics 文件到本地
	err = os.WriteFile(fmt.Sprintf("%s/%s", c.Path, "notion_calendar.ics"), icsData, 0644)
	if err != nil {
		//log.Fatalf("Failed to save ICS file: %v", err)
		return
	}

	//fmt.Println("ICS file generated successfully.")
}

func serveICSFile(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "notion_calendar.ics")
}

func runServer() {

	http.HandleFunc("/calendar.ics", serveICSFile)

	port := ":33189"
	http.ListenAndServe(port, nil)
}
