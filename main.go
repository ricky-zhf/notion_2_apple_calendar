package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

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
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}

// 获取 Notion 数据库数据（支持分页，只获取最近一个月的数据）
func getNotionDatabaseData(token, databaseID string) (*NotionDatabase, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", databaseID)

	var allResults []struct {
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
	}

	var startCursor string
	hasMore := true

	// 计算一个月前的日期
	oneMonthAgo := time.Now().AddDate(0, -1, 0).Format("2006-01-02")

	// 循环获取所有分页数据
	for hasMore {
		// 构建请求体，添加时间过滤器
		requestBody := map[string]interface{}{
			"filter": map[string]interface{}{
				"property": "Time",
				"date": map[string]interface{}{
					"on_or_after": oneMonthAgo,
				},
			},
		}

		// 如果有游标，添加到请求体
		if startCursor != "" {
			requestBody["start_cursor"] = startCursor
		}

		bodyBytes, err := json.Marshal(requestBody)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
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

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		var database NotionDatabase
		err = json.Unmarshal(body, &database)
		if err != nil {
			return nil, err
		}

		// 将当前页的结果添加到总结果中
		allResults = append(allResults, database.Results...)

		// 更新分页信息
		hasMore = database.HasMore
		startCursor = database.NextCursor

		logrus.Infof("已获取 %d 条记录，还有更多数据: %v", len(allResults), hasMore)
	}

	// 返回包含所有结果的数据库对象
	return &NotionDatabase{
		Results:    allResults,
		HasMore:    false,
		NextCursor: "",
	}, nil
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

			// 添加提醒：在事件开始前5分钟
			alarm := event.AddAlarm()
			alarm.SetAction(ical.ActionDisplay)
			alarm.SetTrigger("-PT5M") // 负数表示事件开始前，PT5M = 5分钟
			alarm.SetDescription("notion task: " + result.Properties.Name.Title[0].Text.Content)
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

	//go runCron(conf)

	_ = update(conf)

	_ = syncNotion(conf)
	logrus.Infof("get notion end...")

	runServer(conf)
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

func syncNotion(c Conf) []byte {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("syncNotion panic, err:%v", r)
		}
	}()
	// 替换为你的 Notion API 密钥和数据库 ID
	notionToken := c.Key
	databaseID := c.Databases

	// 获取 Notion 数据库数据
	database, err := getNotionDatabaseData(notionToken, databaseID)
	if err != nil {
		logrus.Errorf("Failed to get Notion database data: %v", err)
		return nil
	}

	// 生成 .ics 文件内容
	icsData, err := generateICS(c, database)
	if err != nil {
		logrus.Errorf("Failed to generate ICS data: %v", err)
		return nil
	}

	// 如果配置了自动推送到 Git，则保存文件并推送
	if c.AutoPushGit {
		//go func() {
		if err = saveAndPushToGit(c, icsData); err != nil {
			logrus.Errorf("Failed to save and push to git: %v", err)
		}
		//}()
	}

	return icsData
}

// 保存 ICS 文件并推送到 GitHub
func saveAndPushToGit(c Conf, icsData []byte) error {
	// 确定文件名
	fileName := c.IcsFileName
	if fileName == "" {
		fileName = "calendar.ics"
	}

	// 确定保存路径
	var filePath string
	if c.Path != "" {
		filePath = filepath.Join(c.Path, fileName)
	} else {
		filePath = fileName
	}

	// 保存文件
	logrus.Infof("开始保存 ICS 文件到: %s", filePath)
	err := os.WriteFile(filePath, icsData, 0644)
	if err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}
	logrus.Infof("ICS 文件保存成功")

	// 推送到 Git
	return pushToGitHub(c, filePath)
}

// 推送到 GitHub
func pushToGitHub(c Conf, filePath string) error {
	// 获取工作目录
	workDir := "."
	if c.Path != "" {
		workDir = c.Path
	}

	// 确定提交信息
	commitMsg := c.GitCommitMsg
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Update calendar.ics - %s", time.Now().Format("2006-01-02 15:04:05"))
	}

	logrus.Infof("开始 Git 操作，工作目录: %s", workDir)

	// 执行 git add
	logrus.Infof("执行: git add %s", filepath.Base(filePath))
	cmd := exec.Command("git", "add", filepath.Base(filePath))
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add 失败: %w, output: %s", err, string(output))
	}

	// 执行 git commit
	logrus.Infof("执行: git commit -m \"%s\"", commitMsg)
	cmd = exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		// 如果没有变更需要提交，不算错误
		if bytes.Contains(output, []byte("nothing to commit")) || bytes.Contains(output, []byte("no changes added")) {
			logrus.Infof("没有需要提交的变更")
			return nil
		}
		return fmt.Errorf("git commit 失败: %w, output: %s", err, string(output))
	}
	logrus.Infof("Git commit 成功: %s", string(output))

	// 执行 git push
	logrus.Infof("执行: git push")
	cmd = exec.Command("git", "push")
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("git push 失败: %v, output: %s", err, string(output))
	}
	logrus.Infof("Git push 成功: %s", string(output))

	return nil
}

func runServer(c Conf) {
	http.HandleFunc("/calendar.ics", func(w http.ResponseWriter, r *http.Request) {
		go update(c)
		b := syncNotion(c)
		if _, err := w.Write(b); err != nil {
			logrus.Errorf("Failed to write response: %v", err)
		}
	})

	port := ":33189"
	if isDev() {
		port = ":33188"
	}
	if err := http.ListenAndServe(port, nil); err != nil {
		logrus.Fatalf("Failed to start server: %v", err)
	}
}
