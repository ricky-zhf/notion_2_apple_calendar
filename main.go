package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"time"

	ical "github.com/arran4/golang-ical"
)

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

// NotionDatabase 定义 Notion 数据库响应结构
//type NotionDatabase struct {
//	Results []struct {
//		Properties struct {
//			Name struct {
//				Title []struct {
//					Text struct {
//						Content string `json:"content"`
//					} `json:"text"`
//				} `json:"title"`
//			} `json:"Name"`
//			Start struct {
//				Date struct {
//					Start string `json:"start"`
//				} `json:"date"`
//			} `json:"Start"`
//			End struct {
//				Date struct {
//					Start string `json:"start"`
//				} `json:"date"`
//			} `json:"End"`
//		} `json:"properties"`
//	} `json:"results"`
//}

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

var (
	layout = "2006-01-02T15:04:05.000Z07:00"
	exeDir string
)

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
	conf, err := InitConfig()
	if err != nil {
		logrus.Fatalf("Failed to init config: %v", err)
	}

	newApp()

	go runCron(conf)

	_ = update(conf)

	syncNotion(conf)
	logrus.Infof("get notion end...")

	go runServer(conf)
	//logrus.Infof("start http end...")
	//
	//ticker := time.NewTicker(1 * time.Minute)
	//defer ticker.Stop()
	//
	//for {
	//	select {
	//	case <-ticker.C:
	//		syncNotion(conf)
	//	}
	//}
}

func syncNotion(c Conf) {
	defer func() {
		_ = recover()
	}()
	// 替换为你的 Notion API 密钥和数据库 ID
	notionToken := c.Key
	databaseID := c.Databases

	// 获取 Notion 数据库数据
	database, err := getNotionDatabaseData(notionToken, databaseID)
	if err != nil {
		logrus.Infof("Failed to get Notion database data: %v", err)
		return
	}

	// 生成 .ics 文件内容
	icsData, err := generateICS(c, database)
	if err != nil {
		logrus.Infof("Failed to generate ICS data: %v", err)
		return
	}

	// 保存 .ics 文件到本地
	err = os.WriteFile(fmt.Sprintf("%s/%s", c.Path, "notion_calendar.ics"), icsData, 0644)
	if err != nil {
		logrus.Infof("Failed to save ICS file: %v", err)
		return
	}
}

func runServer(c Conf) {
	http.HandleFunc("/calendar.ics", func(w http.ResponseWriter, r *http.Request) {
		syncNotion(c)
		path := fmt.Sprintf("%s/%s", exeDir, "notion_calendar.ics")
		http.ServeFile(w, r, path)
	})

	port := ":33189"
	if isDev() {
		port = ":33188"
	}
	if err := http.ListenAndServe(port, nil); err != nil {
		logrus.Fatalf("Failed to start server: %v", err)
	}
}
