package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

type Conf struct {
	Key           string `json:"key"`
	Path          string `json:"path"`
	Databases     string `json:"databases"`
	DefaultPageId string `json:"default_page_id"`
}

var logFile *os.File

func InitConfig() (conf Conf, err error) {
	// 获取可执行文件的绝对路径
	exePath, err := os.Executable()
	if err != nil {
		logrus.Infof("获取可执行文件路径失败: %v", err)
		return
	}
	if isDev() {
		exePath = "./"
	}

	// 获取可执行文件所在目录
	exeDir = filepath.Dir(exePath)

	// 拼接配置文件的绝对路径
	confPath := filepath.Join(exeDir, "conf.json")

	confData, err := os.ReadFile(confPath)
	if err != nil {
		logrus.Infof("读取配置文件失败: %v", err)
		return
	}
	err = json.Unmarshal(confData, &conf)
	if err != nil {
		logrus.Infof("解析配置文件失败: %v", err)
		return
	}

	initLogger(exeDir)
	return
}

func initLogger(path string) {
	// 创建日志文件
	logPath := filepath.Join(path, "notion2apple-prod.log")
	if isDev() {
		logPath = "notion2apple-dev.log"
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logrus.Fatal("无法打开日志文件:", err)
	}

	logrus.SetOutput(file)
	logrus.SetFormatter(&logrus.JSONFormatter{}) // JSON 格式
	logrus.SetLevel(logrus.InfoLevel)            // 设置日志级别

	logFile = file
	return
}

type App struct {
	shutdownSignal chan struct{}
	wg             sync.WaitGroup
}

func newApp() {
	app := &App{
		shutdownSignal: make(chan struct{}),
	}
	app.StartBackgroundTask()
	app.SetupGracefulShutdown()
}

func (a *App) StartBackgroundTask() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case <-a.shutdownSignal:
				logrus.Infof("后台任务收到关闭信号，正在清理...")
				logFile.Close()
				return
			}
		}
	}()
}

// SetupGracefulShutdown 注册优雅关闭逻辑
func (a *App) SetupGracefulShutdown() {
	// 监听终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	go func() {
		sig := <-sigChan
		logrus.Infof("收到终止信号: %v，开始关闭程序...", sig)

		close(a.shutdownSignal) // 通知所有组件停止

		// 设置关闭超时（防止死锁）
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 等待所有任务退出
		done := make(chan struct{})
		go func() {
			a.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			logrus.Infof("警告: 关闭超时，强制退出")
		}

		os.Exit(0)
	}()
}

func isDev() bool {
	return os.Getenv("env") == "dev"
}
