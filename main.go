package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hekmon/transmissionrpc/v3"
)

// 定义正则表达式
var (
	// 匹配种子名称格式的正则表达式
	torrentNameRegex = regexp.MustCompile(`^(.+) - S(\d+)$`)
	episodeRegex     = regexp.MustCompile(`【(\d+(?:\.5)?)】|\[(\d+(?:\.5)?)]|第(\d+(?:\.5)?)集| (\d+(?:\.5)?)|E(\d+(?:\.5)?)`)
)

type Config struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

func main() {
	// 读取配置文件
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatal("读取配置文件失败:", err)
	}

	var config Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		log.Fatal("解析配置文件失败:", err)
	}

	// 构建 Transmission RPC URL
	endpoint, err := url.Parse(fmt.Sprintf("http://%s:%s@%s:%d/transmission/rpc",
		config.Username,
		config.Password,
		config.Host,
		config.Port))
	if err != nil {
		log.Fatal(err)
	}

	client, err := transmissionrpc.New(endpoint, nil)
	if err != nil {
		log.Fatal(err)
	}

	// 获取所有种子
	torrents, err := client.TorrentGet(context.Background(), []string{"id", "hashString", "name", "labels", "downloadDir"}, nil)
	if err != nil {
		log.Fatal(err)
	}

	// 处理每个种子
	for _, torrent := range torrents {
		log.Printf("正在处理种子: %s", *torrent.Name)

		// 检查是否包含 collection 标签
		hasCollectionLabel := false
		labelIndex := -1
		for i, label := range torrent.Labels {
			if strings.Contains(strings.ToLower(label), "collection") {
				hasCollectionLabel = true
				labelIndex = i
				log.Printf("找到 collection 标签: %s", label)
				break
			}
		}

		if !hasCollectionLabel {
			log.Printf("跳过未标记 collection 的种子: %s", *torrent.Name)
			continue
		}

		// 检查种子名称是否符合格式
		match := torrentNameRegex.FindStringSubmatch(*torrent.Name)
		if match == nil {
			log.Printf("种子名称格式不符合要求: %s", *torrent.Name)
			continue
		}

		seriesName := match[1]
		seasonNum := match[2]

		// 获取种子的详细信息，包括文件列表
		torrentInfo, err := client.TorrentGet(context.Background(),
			[]string{"id", "name", "files", "downloadDir"},
			[]int64{*torrent.ID})
		if err != nil {
			log.Printf("获取种子详细信息失败: %v", err)
			continue
		}

		if len(torrentInfo) == 0 {
			log.Printf("未找到种子信息")
			continue
		}

		currentTorrent := torrentInfo[0]

		// 添加一个标志来跟踪是否所有文件都成功重命名
		allFilesRenamed := true

		// 处理种子中的每个文件
		for _, file := range currentTorrent.Files {
			log.Printf("处理文件: %s", file.Name)

			// 提取集数
			episodeMatch := episodeRegex.FindStringSubmatch(file.Name)
			if len(episodeMatch) == 0 {
				log.Printf("无法从文件名提取集数: %s", file.Name)
				continue
			}

			// 找到第一个非空的匹配组
			var episodeNum string
			for i := 1; i < len(episodeMatch); i++ {
				if episodeMatch[i] != "" {
					episodeNum = episodeMatch[i]
					break
				}
			}

			// 获取文件扩展名
			ext := filepath.Ext(file.Name)

			// 构建新的文件名
			newBaseName := fmt.Sprintf("%s S%sE%s%s",
				seriesName,
				fmt.Sprintf("%02s", seasonNum),
				fmt.Sprintf("%02s", episodeNum),
				ext,
			)

			// 获取当前文件的路径和新的名称
			oldPath := filepath.ToSlash(file.Name) // 当前文件的相对路径
			newName := newBaseName                 // 只需要新的文件名，不是完整路径

			oldFileName := filepath.Base(oldPath)
			if oldFileName == newName {
				log.Printf("文件名相同,跳过重命名: %s", oldPath)
				continue
			}
			log.Printf("重命名文件: %s -> %s", oldPath, newName)

			// 使用正确的参数调用重命名方法
			err = client.TorrentRenamePath(context.Background(), *currentTorrent.ID, oldPath, newName)
			if err != nil {
				log.Printf("重命名文件失败: %v", err)
				allFilesRenamed = false
				break
			}

			log.Printf("成功重命名文件: %s", newName)
		}

		// 只有当所有文件都成功重命名后才更新标签
		if allFilesRenamed {
			// 更新标签
			newLabels := make([]string, len(torrent.Labels))
			copy(newLabels, torrent.Labels)
			newLabels[labelIndex] = "rename"

			payload := transmissionrpc.TorrentSetPayload{
				IDs:    []int64{*torrent.ID},
				Labels: newLabels,
			}

			if err := client.TorrentSet(context.Background(), payload); err != nil {
				log.Printf("更新种子标签失败: %v", err)
				continue
			}

			log.Printf("已更新种子 %s 的标签", *torrent.Name)
		}
	}
}
