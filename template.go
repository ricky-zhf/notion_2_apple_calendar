package main

import (
	"context"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"time"

	"github.com/jomei/notionapi"
)

// 创建页面并设置时间属性为当前时间
//func CreatePageWithCurrentTime(
//	client *notionapi.Client,
//	databaseID string,
//	timePropName string,
//) error {
//	// 获取当前时间（带时区信息）
//	now := time.Now().UTC().Format(time.RFC3339) // Notion要求ISO 8601格式
//
//	// 构造请求体
//	req := &notionapi.PageCreateRequest{
//		Parent: notionapi.Parent{
//			DatabaseID: notionapi.DatabaseID(databaseID),
//		},
//		Properties: notionapi.Properties{
//			timePropName: notionapi.DateProperty{
//				Type: notionapi.PropertyTypeDate,
//				Date: &notionapi.DateObject{
//					Start: notionapi.Date(time.Now()),
//				},
//			},
//		},
//	}
//
//	// 调用API创建页面
//	_, err := client.Page.Create(context.Background(), req)
//	return err
//}

func UpdatePageTime(client *notionapi.Client, pageID string, timePropName string) error {
	//now := time.Now().UTC().Format(time.RFC3339)
	date := notionapi.Date(time.Now())
	req := &notionapi.PageUpdateRequest{
		Properties: notionapi.Properties{
			timePropName: notionapi.DateProperty{
				Type: notionapi.PropertyTypeDate,
				Date: &notionapi.DateObject{
					Start: &date,
					End:   &date,
				},
			},
		},
	}

	_, err := client.Page.Update(context.Background(), notionapi.PageID(pageID), req)
	return err
}

func update(conf Conf) {
	// 初始化Notion客户端
	client := notionapi.NewClient(notionapi.Token(conf.Key))

	// 执行创建操作
	err := UpdatePageTime(
		client,
		conf.DefaultPageId,
		"Time", // 替换为你的时间属性名称
	)
	if err != nil {
		logrus.Errorf("Update default template success failed, err:%v", err)
		return
	}
	logrus.Infof("Update default template success")
}

func runCron(conf Conf) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	c := cron.New(cron.WithLocation(location), cron.WithSeconds())

	// 2. 添加定时任务（每天 0 点 1 分执行, 0 1 * * * *
	// @every 1m
	execTime := "0 1 0 * * *"
	if isDev() {
		execTime = "@every 1m"
	}
	_, err := c.AddFunc(execTime, func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Infof("定时任务执行失败: %v", r)
			}
		}()
		update(conf)
	})

	if err != nil {
		logrus.Errorf("添加定时任务失败: %v", err)
	}

	c.Start()
	logrus.Infof("定时任务已启动，等待执行...")
	select {}
}
