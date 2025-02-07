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
	"slices"
	"strings"

	"github.com/hekmon/transmissionrpc/v3"
)

type Config struct {
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	Host            string   `json:"host"`
	Port            int      `json:"port"`
	SeasonPathRegex string   `json:"seasonPathRegex"`
	EpisodeRegex    []string `json:"episodeRegex"`
	Ext             []string `json:"ext"`
	ExtSubs         []string `json:"extSubs"`
	LangRegex       string   `json:"langRegex"`
}

func main() {
	// 获取可执行文件的路径
	executablePath, err := os.Executable()
	if err != nil {
		log.Fatal("获取可执行文件路径失败:", err)
	}

	// 尝试从二进制文件所在的目录读取配置文件
	configFilePath := filepath.Join(filepath.Dir(executablePath), "config.json")

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		// 如果二进制目录中没有配置文件，则尝试从当前工作目录读取
		log.Printf("二进制目录中的配置文件不存在，尝试从当前工作目录读取")
		configFilePath = "config.json"
	} else if err != nil {
		log.Fatal("检查配置文件时发生错误:", err)
	}

	configFile, err := os.ReadFile(configFilePath)
	if err != nil {
		log.Fatal("读取配置文件失败:", err)
	}

	var config Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		log.Fatal("解析配置文件失败:", err)
	}

	// 尝试编译正则表达式
	seasonPathRegex := regexp.MustCompile(config.SeasonPathRegex)
	langRegex := regexp.MustCompile(config.LangRegex)
	// 检查正则表达式是否有效
	if seasonPathRegex == nil {
		log.Fatal("季度路径正则表达式无效")
	}

	if len(config.EpisodeRegex) == 0 {
		log.Fatal("集数正则表达式无效")
	}

	if langRegex == nil {
		log.Fatal("多国语言正则表达式无效")
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

		bgmName := *torrent.Name // 动漫名称
		seasonNum := "01"        // 默认为第一季

		// 处理手动指定季数
		if seasonMatch := seasonPathRegex.FindStringSubmatch(bgmName); seasonMatch != nil {
			seasonNum = fmt.Sprintf("%02s", seasonMatch[1])
			bgmName = strings.Replace(bgmName, seasonMatch[0], "", 1)
			// 再去掉前后空格
			bgmName = strings.TrimSpace(bgmName)
		}

		// 添加一个标志来跟踪是否所有文件都成功重命名
		allFilesRenamed := true

		// 处理种子中的每个文件
		for i, file := range currentTorrent.Files {
			// 检查文件是否被选择下载
			if !currentTorrent.Wanted[i] {
				// log.Printf("跳过未选择下载的文件: %s", file.Name)
				continue
			}

			oldPath := file.Name                             // 文件相对下载目录的路径
			oldPath = strings.ReplaceAll(oldPath, "\\", "/") // 确保使用正斜杠
			oldBaseName := filepath.Base(oldPath)            // 文件名
			ext := filepath.Ext(oldPath)                     // 文件扩展名
			log.Printf("处理文件: %s", oldPath)

			// 如果文件路径包含的/超过两个,则跳过
			if strings.Count(oldPath, "/") > 2 {
				log.Printf("跳过多层文件夹: %s", oldPath)
				continue
			}

			// 为每个文件提取季度信息
			if seasonMatch := seasonPathRegex.FindStringSubmatch(oldPath); seasonMatch != nil {
				seasonNum = fmt.Sprintf("%02s", seasonMatch[1])
				// log.Printf("从文件路径中提取到季度: %s", seasonNum)
			} else {
				if strings.Count(oldPath, "/") > 1 {
					log.Printf("未找到季度,跳过文件夹: %s", oldPath)
					continue
				}
			}

			// 如果文件不是视频文件或者字幕文件, 则跳过
			if !slices.Contains(config.Ext, ext) && !slices.Contains(config.ExtSubs, ext) {
				log.Printf("跳过非视频或字幕文件: %s", oldPath)
				continue
			}

			// 检查文件名是否包含 SxxExx 格式
			var episodeNum string
			for _, regex := range config.EpisodeRegex { // 遍历正则表达式数组
				episodeMatch := regexp.MustCompile(regex).FindStringSubmatch(oldBaseName)
				if len(episodeMatch) > 0 {
					episodeNum = episodeMatch[1] // 使用捕获的集数
					break                        // 找到后退出循环
				}
			}
			if episodeNum == "" {
				log.Printf("无法从文件名提取集数: %s", oldBaseName)
				continue
			}

			// log.Printf("从文件名 %s 中提取到集数: %s", oldBaseName, episodeNum)

			// 构建新文件名时保持相同的目录结构
			// 如果文件是字幕文件, 则需要保留多语言后缀
			lang := ""
			if slices.Contains(config.ExtSubs, ext) {
				oldBaseNameWithoutExt := oldBaseName[:len(oldBaseName)-len(ext)] // 去掉扩展名
				if langMatch := langRegex.FindStringSubmatch(oldBaseNameWithoutExt); langMatch != nil {
					lang = langMatch[0]
				}
			}

			newBaseName := fmt.Sprintf("%s S%sE%s%s%s",
				bgmName,
				seasonNum,
				fmt.Sprintf("%02s", episodeNum),
				lang,
				ext,
			)

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
