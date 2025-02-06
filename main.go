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

type Config struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	SeasonPathRegex string `json:"seasonPathRegex"`
	EpisodeRegex    string `json:"episodeRegex"`
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

	// 尝试编译正则表达式
	seasonPathRegex := regexp.MustCompile(config.SeasonPathRegex)
	episodeRegex := regexp.MustCompile(config.EpisodeRegex)

	// 检查正则表达式是否有效
	if seasonPathRegex == nil {
		log.Fatal("季度路径正则表达式无效")
	}

	if episodeRegex == nil {
		log.Fatal("集数正则表达式无效")
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
			// log.Printf("跳过未标记 collection 的种子: %s", *torrent.Name)
			continue
		}

		// 获取种子的详细信息，包括文件列表
		torrentInfo, err := client.TorrentGet(context.Background(),
			[]string{"id", "name", "files", "downloadDir", "wanted", "priorities"},
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
		for i, file := range currentTorrent.Files {
			// 检查文件是否被选择下载
			if !currentTorrent.Wanted[i] {
				// log.Printf("跳过未选择下载的文件: %s", file.Name)
				continue
			}
			oldPath := file.Name
			oldBaseName := filepath.Base(oldPath)
			log.Printf("处理文件: %s", oldPath)

			// 为每个文件提取季度信息
			seasonNum := "01" // 默认为第一季
			if seasonMatch := seasonPathRegex.FindStringSubmatch(oldPath); seasonMatch != nil {
				seasonNum = fmt.Sprintf("%02s", seasonMatch[1])
				// log.Printf("从文件路径中提取到季度: %s", seasonNum)
			}

			// 检查文件名是否包含 SxxExx 格式
			var episodeNum string
			specialEpisodeMatch := regexp.MustCompile(`S\d+E(\d+)`).FindStringSubmatch(oldBaseName)
			if specialEpisodeMatch != nil {
				episodeNum = specialEpisodeMatch[1] // 使用捕获的集数
			} else {
				episodeMatch := episodeRegex.FindStringSubmatch(oldBaseName)
				if len(episodeMatch) == 0 {
					log.Printf("无法从文件名提取集数: %s", oldBaseName)
					continue
				}

				// 找到第一个非空的匹配组
				for i := 1; i < len(episodeMatch); i++ {
					if episodeMatch[i] != "" {
						episodeNum = episodeMatch[i]
						break
					}
				}
			}

			// log.Printf("从文件名 %s 中提取到集数: %s", oldBaseName, episodeNum)

			// 获取文件扩展名
			ext := filepath.Ext(oldPath)

			// 构建新文件名时保持相同的目录结构
			newBaseName := fmt.Sprintf("%s S%sE%s%s",
				*torrent.Name,
				seasonNum,
				fmt.Sprintf("%02s", episodeNum),
				ext,
			)

			// 确保使用正斜杠
			oldPath = strings.ReplaceAll(oldPath, "\\", "/")

			// 检查文件名是否相同
			if oldBaseName == newBaseName {
				log.Printf("文件名相同,跳过重命名")
				continue
			}

			log.Printf("重命名文件: %s -> %s", oldBaseName, newBaseName)

			// 只传入文件名进行重命名
			// 这里前面传递的是路径，后面传递的是文件名
			err = client.TorrentRenamePath(context.Background(), *currentTorrent.ID, oldPath, newBaseName)
			if err != nil {
				log.Printf("重命名文件失败: %v", err)
				allFilesRenamed = false
				continue
			}

			log.Printf("重命名文件成功")
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
